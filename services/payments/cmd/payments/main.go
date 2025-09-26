package main

import (
	"context"
	"github.com/twmb/franz-go/pkg/kgo"
	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/payments/config"
	"goshop/services/payments/internal/consumer"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config
	cfg := config.NewConfig()

	// Logger
	log := logger.NewPrettyLogger(cfg.Logger)

	// Postgres
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Error("postgres: connect failed", "host", cfg.Postgres.Host, "port", cfg.Postgres.Port, "db", cfg.Postgres.DBName, "err", err)
		return
	}
	defer pool.Close()
	log.Info("postgres: connected")

	// Kafka consumer client
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.DialTimeout(2 * time.Second),
		kgo.ClientID("payments"),
		kgo.ConsumerGroup(cfg.Consumer.Group),
		kgo.ConsumeTopics(cfg.Consumer.Topic),
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		log.Error("kafka: client init failed", "err", err)
		os.Exit(1)
	}
	defer cl.Close()

	// Processor
	proc := consumer.NewProcessor(log, pool, cfg.Outbox.Topic)

	// Runner
	rcfg := consumer.Config{
		Group:            cfg.Consumer.Group,
		Topic:            cfg.Consumer.Topic,
		SessionTimeout:   cfg.Consumer.SessionTimeout,
		RebalanceTimeout: cfg.Consumer.RebalanceTimeout,
	}
	r := consumer.New(log, pool, cl, rcfg, proc)

	if err := r.Run(ctx); err != nil && err != context.Canceled {
		log.Error("payments-consumer: stopped with error", "err", err)
		os.Exit(1)
	}
	log.Info("payments-consumer: stopped")
}
