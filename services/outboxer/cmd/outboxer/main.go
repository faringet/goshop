package main

import (
	"context"
	"errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/outboxer/config"
	"goshop/services/outboxer/internal/worker"
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
	log.Info("postgres: connected")

	// Kafka client
	kopts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.DialTimeout(2 * time.Second),
		kgo.RequestTimeoutOverhead(5 * time.Second),
	}
	kc, err := kgo.NewClient(kopts...)
	if err != nil {
		log.Error("kafka: client init failed", "err", err)
		os.Exit(1)
	}
	defer kc.Close()

	// Админская Kafka
	adm := kadm.NewClient(kc)
	defer adm.Close()

	// Smoke че там по топикам
	mdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	md, err := adm.Metadata(mdCtx)
	if err != nil {
		log.Error("kafka: metadata failed", "err", err)
		os.Exit(1)
	}
	log.Info("kafka: metadata ok",
		"brokers", len(md.Brokers),
		"topics", len(md.Topics),
	)

	wcfg := worker.Config{
		BatchSize:      cfg.Worker.BatchSize,
		PollInterval:   cfg.Worker.PollInterval,
		ProduceTimeout: cfg.Worker.ProduceTimeout,
		MaxRetries:     cfg.Worker.MaxRetries,
		BackoffBaseMS:  cfg.Worker.BackoffBaseMS,
	}

	wr := worker.New(pool, kc, wcfg)

	log.Info("outboxer: starting loop",
		"batch_size", wcfg.BatchSize,
		"poll", wcfg.PollInterval,
	)

	log.Info("outboxer: starting loop", "batch_size", wcfg.BatchSize, "poll", wcfg.PollInterval)
	if err := wr.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("outboxer: stopped with error", "err", err)
		os.Exit(1)
	}
	log.Info("outboxer: stopped")

}
