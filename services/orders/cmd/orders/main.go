package main

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"goshop/pkg/httpx"
	"goshop/pkg/jwtauth"
	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/orders/config"
	"goshop/services/orders/internal/adapters/http"
	"goshop/services/orders/internal/adapters/repo/orderpg"
	"goshop/services/orders/internal/consumer"
	grpcsvr "goshop/services/orders/internal/grpc"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	start := time.Now()

	// OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config
	cfg := config.New()

	// Logger
	log := logger.NewPrettyLogger(cfg.Logger)

	// Postgres
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Error("postgres: connect failed", "host", cfg.Postgres.Host, "port", cfg.Postgres.Port, "db", cfg.Postgres.DBName, "err", err)
		return
	}
	defer pool.Close()

	repo := orderpg.NewRepo(pool)
	_ = repo

	// JWT
	jwtm := jwtauth.New(jwtauth.Config{
		Secret:     cfg.JWT.Secret,
		Issuer:     cfg.JWT.Issuer,
		AccessTTL:  cfg.JWT.AccessTTL,
		RefreshTTL: cfg.JWT.RefreshTTL,
	})
	log.Info("jwt: verifier initialized", "issuer", cfg.JWT.Issuer)

	// HTTP module
	ordersHTTP := httpadp.NewModule(log, pool, repo, jwtm)

	// HTTP server with modules
	srv := httpx.NewServer(cfg.HTTP, log,
		httpx.WithModules(ordersHTTP),
	)

	// Kafka consumer (orders читает payments.events)
	kopts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.DialTimeout(2 * time.Second),
		kgo.ClientID("orders"),
		kgo.ConsumerGroup(cfg.Consumer.Group),
		kgo.ConsumeTopics(cfg.Consumer.Topic),
	}
	kc, err := kgo.NewClient(kopts...)
	if err != nil {
		log.Error("kafka: client init failed", "err", err)
		os.Exit(1)
	}
	defer kc.Close()

	// Redis
	rds := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rds.Close()
	if err := rds.Ping(ctx).Err(); err != nil {
		log.Error("redis: connect failed", "addr", cfg.Redis.Addr, "err", err)
		return
	}
	// Процессор: применяем payment.confirmed/failed → orders.status
	proc := consumer.NewProcessor(log, pool, rds, cfg.Redis.TTLStatus)

	// Раннер консюмера
	rcfg := consumer.Config{
		Group:            cfg.Consumer.Group,
		Topic:            cfg.Consumer.Topic,
		SessionTimeout:   cfg.Consumer.SessionTimeout,
		RebalanceTimeout: cfg.Consumer.RebalanceTimeout,
	}
	r := consumer.New(log, pool, kc, rcfg, proc)

	// Запускаем консюмер в горутине
	go func() {
		if err := r.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("orders-consumer: stopped with error", "err", err)
			stop()
		}
	}()

	// HTTP Listen
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http: listen failed", "err", err)
			stop()
		}
	}()

	ordersGRPC := grpcsvr.Options{
		Addr:   cfg.GRPC.Addr,
		Logger: log,
		Repo:   repo,
	}
	go func() {
		if err := grpcsvr.Start(ctx, ordersGRPC); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("orders-grpc: stopped with error", "err", err)
			stop()
		}
	}()

	// Wait for signal
	<-ctx.Done()
	log.Info("shutdown: received signal, stopping...")

	// graceful HTTP
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http: graceful shutdown failed", "err", err)
	} else {
		log.Info("http: server stopped cleanly")
	}

	log.Info("bye", "uptime", time.Since(start), "pid", os.Getpid())
}
