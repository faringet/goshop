package main

import (
	"context"
	"errors"
	"github.com/twmb/franz-go/pkg/kgo"
	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/orders/config"
	httpserver "goshop/services/orders/internal/adapters/http"
	"goshop/services/orders/internal/adapters/repo/orderpg"
	"goshop/services/orders/internal/consumer"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
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

	srv := httpserver.NewBuilder(cfg.HTTP, log).
		WithDB(pool).
		WithDefaultEndpoints().
		WithOrders(repo).
		Build()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("http: listen failed", "err", err)
			stop()
		}
	}()

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

	// Процессор: применяем payment.confirmed/failed → orders.status
	proc := consumer.NewProcessor(log, pool)

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

	// Ожидаем сигнал
	<-ctx.Done()
	log.Info("shutdown: stopping...")

	// Аккуратный стоп HTTP
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	log.Info("stopped cleanly")
}
