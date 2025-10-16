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
	"sync"
	"syscall"
	"time"
)

func main() {
	start := time.Now()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config
	cfg := config.New()

	// Logger
	log := logger.NewPrettyLogger(cfg.Logger)
	log.Info("boot: starting outboxer", "workers", len(cfg.AllWorkers()))

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
		go func(tbl string) {
			defer wg.Done()
			log.Info("outboxer: worker starting", "table", tbl, "poll", wc.PollInterval, "batch", wc.BatchSize)
			if err := wr.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("outboxer: worker stopped with error", "table", tbl, "err", err)
				errCh <- err
				cancelAll()
				return
			}
			log.Info("outboxer: worker stopped", "table", tbl)
		}(w.OutboxTable)
	}

	// Ждём либо первый фатальный еррор, либо сигнал
	select {
	case <-ctx.Done():
		log.Info("shutdown: stopping workers...")
	case err := <-errCh:
		log.Error("shutdown: error from worker", "err", err)
	}

	// Тушим и ждём
	cancelAll()
	wg.Wait()
	log.Info("bye", "uptime", time.Since(start))

}
