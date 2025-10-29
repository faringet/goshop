package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/payments/config"
	"goshop/services/payments/internal/consumer"
)

func main() {
	start := time.Now()

	// OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config + Logger (единый стиль)
	cfg := config.New()
	log := logger.NewLogger(cfg.Logger)
	slog.SetDefault(log)

	if err := cfg.Validate(); err != nil {
		log.Error("config: invalid", slog.Any("err", err))
		return
	}

	log.Info("payments.consumer: starting",
		slog.String("kafka.topic", cfg.Consumer.Topic),
		slog.String("group", cfg.Consumer.Group),
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

	// Kafka consumer client
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.DialTimeout(2 * time.Second),
		kgo.ClientID("payments"),
		kgo.ConsumerGroup(cfg.Consumer.Group),
		kgo.ConsumeTopics(cfg.Consumer.Topic),
	}
	kcStart := time.Now()
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		log.Error("kafka: client init failed", slog.Any("err", err))
		os.Exit(1)
	}
	log.Info("kafka: client ready",
		slog.String("group", cfg.Consumer.Group),
		slog.String("topic", cfg.Consumer.Topic),
		slog.Int64("latency_ms", time.Since(kcStart).Milliseconds()),
	)
	defer cl.Close()

	// Processor & Runner
	proc := consumer.NewProcessor(log, pool, cfg.Outbox.Topic)
	rcfg := consumer.Config{
		Group:            cfg.Consumer.Group,
		Topic:            cfg.Consumer.Topic,
		SessionTimeout:   cfg.Consumer.SessionTimeout,
		RebalanceTimeout: cfg.Consumer.RebalanceTimeout,
	}
	r := consumer.New(log, pool, cl, rcfg, proc)

	// Run (blocking)
	if err := r.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("payments.consumer: stopped with error", slog.Any("err", err))
		os.Exit(1)
	}
	log.Info("payments.consumer: stopped",
		slog.Int64("uptime_ms", time.Since(start).Milliseconds()),
	)
}
