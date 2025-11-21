// services/outboxer/cmd/outboxer/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/outboxer/config"
	"goshop/services/outboxer/internal/worker"
)

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

	log.Info("outboxer: starting",
		slog.Int("workers", len(cfg.AllWorkers())),
		slog.Int("kafka_brokers", len(cfg.Kafka.Brokers)),
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

	// Kafka client
	kopts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.DialTimeout(2 * time.Second),
		kgo.RequestTimeoutOverhead(5 * time.Second),
	}
	kcStart := time.Now()
	kc, err := kgo.NewClient(kopts...)
	if err != nil {
		log.Error("kafka: client init failed", slog.Any("err", err))
		return
	}
	log.Info("kafka: client ready",
		slog.Int("brokers", len(cfg.Kafka.Brokers)),
		slog.Int64("latency_ms", time.Since(kcStart).Milliseconds()),
	)
	defer kc.Close()

	// Админская Kafka
	adm := kadm.NewClient(kc)
	defer adm.Close()

	mdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	md, err := adm.Metadata(mdCtx)
	cancel()
	if err != nil {
		log.Error("kafka: metadata failed", slog.Any("err", err))
		return
	}
	log.Info("kafka: metadata ok",
		slog.Int("brokers", len(md.Brokers)),
		slog.Int("topics", len(md.Topics)),
	)

	// Поднимаем все воркеры параллельно
	var (
		wg        sync.WaitGroup
		errCh     = make(chan error, len(cfg.AllWorkers()))
		cancelAll context.CancelFunc
	)
	ctx, cancelAll = context.WithCancel(ctx)
	defer cancelAll()

	for _, w := range cfg.AllWorkers() {
		wc := worker.Config{
			OutboxTable:    w.OutboxTable,
			BatchSize:      w.BatchSize,
			PollInterval:   w.PollInterval,
			ProduceTimeout: w.ProduceTimeout,
			MaxRetries:     w.MaxRetries,
			BackoffBaseMS:  w.BackoffBaseMS,
		}
		wr := worker.New(pool, kc, wc)

		wg.Add(1)
		go func(tbl string, wc worker.Config) {
			defer wg.Done()
			log.Info("outboxer.worker: starting",
				slog.String("table", tbl),
				slog.Int("batch_size", wc.BatchSize),
				slog.Int64("poll_interval_ms", wc.PollInterval.Milliseconds()),
			)
			if err := wr.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("outboxer.worker: stopped with error",
					slog.String("table", tbl),
					slog.Any("err", err),
				)
				errCh <- err
				cancelAll()
				return
			}
			log.Info("outboxer.worker: stopped", slog.String("table", tbl))
		}(w.OutboxTable, wc)
	}

	// Ждём либо первый фатальный еррор, либо сигнал
	select {
	case <-ctx.Done():
		log.Info("outboxer: shutdown: signal received")
	case err := <-errCh:
		log.Error("outboxer: shutdown: error from worker", slog.Any("err", err))
	}

	// Тушим и ждём
	cancelAll()
	wg.Wait()

	log.Info("bye",
		slog.Int64("uptime_ms", time.Since(start).Milliseconds()),
	)
}
