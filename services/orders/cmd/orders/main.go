package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"

	"goshop/pkg/httpx"
	"goshop/pkg/jwtauth"
	"goshop/pkg/logger"
	"goshop/pkg/postgres"

	"goshop/services/orders/config"
	httpadp "goshop/services/orders/internal/adapters/http"
	"goshop/services/orders/internal/adapters/repo/orderpg"
	"goshop/services/orders/internal/consumer"
	grpcsvr "goshop/services/orders/internal/grpc"
)

const shutdownHTTP = 10 * time.Second

func main() {
	start := time.Now()

	// OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config + Logger
	cfg := config.New()
	log := logger.NewLogger(cfg.Logger)
	slog.SetDefault(log)

	if err := cfg.Validate(); err != nil {
		log.Error("config: invalid", slog.Any("err", err))
		return
	}

	log.Info("orders: starting",
		slog.String("http.addr", cfg.HTTP.Addr),
		slog.String("grpc.addr", cfg.GRPC.Addr),
		slog.String("kafka.topic", cfg.Consumer.Topic),
		slog.String("redis.addr", cfg.Redis.Addr),
	)

	// Postgres
	pgStart := time.Now()
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Error("postgres: connect failed",
			slog.String("host", cfg.Postgres.Host),
			slog.Int("port", cfg.Postgres.Port),
			slog.String("db", cfg.Postgres.DBName),
			slog.Any("err", err),
		)
		return
	}
	log.Info("postgres: connected",
		slog.String("dsn", fmt.Sprintf("%s@%s:%d/%s", cfg.Postgres.User, cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.DBName)),
		slog.Int64("latency_ms", time.Since(pgStart).Milliseconds()),
	)
	defer pool.Close()

	// Repository
	repo := orderpg.NewRepo(pool)

	// JWT
	jwtm := jwtauth.New(jwtauth.Config{
		Secret:     cfg.JWT.Secret,
		Issuer:     cfg.JWT.Issuer,
		AccessTTL:  cfg.JWT.AccessTTL,
		RefreshTTL: cfg.JWT.RefreshTTL,
		// если в конфиге есть audience — можно добавить, но поведение не меняем
	})
	log.Info("jwt: manager initialized", slog.String("issuer", cfg.JWT.Issuer))

	// HTTP module + server
	ordersHTTP := httpadp.NewModule(log, pool, repo, jwtm)
	srv := httpx.NewServer(cfg.HTTP, log, httpx.WithModules(ordersHTTP))

	// Kafka client (read payments.events)
	kopts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.DialTimeout(2 * time.Second),
		kgo.ClientID("orders"),
		kgo.ConsumerGroup(cfg.Consumer.Group),
		kgo.ConsumeTopics(cfg.Consumer.Topic),
	}
	kcStart := time.Now()
	kc, err := kgo.NewClient(kopts...)
	if err != nil {
		log.Error("kafka: client init failed", slog.Any("err", err))
		return
	}
	log.Info("kafka: client ready",
		slog.String("group", cfg.Consumer.Group),
		slog.String("topic", cfg.Consumer.Topic),
		slog.Int64("latency_ms", time.Since(kcStart).Milliseconds()),
	)
	defer kc.Close()

	// Redis
	rdStart := time.Now()
	rds := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rds.Ping(ctx).Err(); err != nil {
		log.Error("redis: connect failed", slog.String("addr", cfg.Redis.Addr), slog.Any("err", err))
		return
	}
	log.Info("redis: connected",
		slog.String("addr", cfg.Redis.Addr),
		slog.Int64("latency_ms", time.Since(rdStart).Milliseconds()),
	)
	defer func() { _ = rds.Close() }()

	// Processor
	proc := consumer.NewProcessor(log, pool, rds, cfg.Redis.TTLStatus)

	// Runner
	rcfg := consumer.Config{
		Group:            cfg.Consumer.Group,
		Topic:            cfg.Consumer.Topic,
		SessionTimeout:   cfg.Consumer.SessionTimeout,
		RebalanceTimeout: cfg.Consumer.RebalanceTimeout,
	}
	r := consumer.New(log, pool, kc, rcfg, proc)

	// Consumer
	go func() {
		if err := r.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("orders-consumer: stopped with error", slog.Any("err", err))
			stop()
		}
	}()

	// HTTP
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http: listen failed", slog.Any("err", err))
			stop()
		}
	}()

	// gRPC
	go func() {
		if err := grpcsvr.Start(ctx, grpcsvr.Options{
			Addr:   cfg.GRPC.Addr,
			Logger: log,
			Repo:   repo,
		}); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("orders-grpc: stopped with error", slog.Any("err", err))
			stop()
		}
	}()

	// Wait for signal
	<-ctx.Done()
	log.Info("orders: shutdown: signal received")

	// graceful HTTP
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownHTTP)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http: graceful shutdown failed", slog.Any("err", err))
	} else {
		log.Info("http: server stopped cleanly")
	}

	log.Info("bye",
		slog.Int("pid", os.Getpid()),
		slog.Int64("uptime_ms", time.Since(start).Milliseconds()),
	)
}
