package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Config struct {
	Group            string
	Topic            string
	SessionTimeout   time.Duration
	RebalanceTimeout time.Duration
}

type Runner struct {
	log  *slog.Logger
	db   *pgxpool.Pool
	kc   *kgo.Client
	cfg  Config
	done chan struct{}

	proc *Processor
}

func New(log *slog.Logger, db *pgxpool.Pool, kc *kgo.Client, cfg Config, proc *Processor) *Runner {
	return &Runner{
		log:  log,
		db:   db,
		kc:   kc,
		cfg:  cfg,
		done: make(chan struct{}),
		proc: proc,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	r.log.Info("payments.consumer: starting",
		slog.String("group", r.cfg.Group),
		slog.String("topic", r.cfg.Topic),
		slog.Int64("session_timeout_ms", r.cfg.SessionTimeout.Milliseconds()),
		slog.Int64("rebalance_timeout_ms", r.cfg.RebalanceTimeout.Milliseconds()),
	)
	defer close(r.done)

	for {
		select {
		case <-ctx.Done():
			r.log.Info("payments.consumer: stopping", slog.String("reason", "context done"))
			return ctx.Err()
		default:
		}

		fetches := r.kc.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				r.log.Warn("payments.consumer.kafka: fetch error",
					slog.String("topic", fe.Topic),
					slog.Int64("partition", int64(fe.Partition)),
					slog.Any("err", fe.Err),
				)
			}
			continue
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()

			if err := r.proc.ProcessRecord(ctx, rec); err != nil && !errors.Is(err, context.Canceled) {
				r.log.Error("payments.consumer.processor: error",
					slog.String("topic", rec.Topic),
					slog.Int64("partition", int64(rec.Partition)),
					slog.Int64("offset", rec.Offset),
					slog.Any("err", err),
				)
				continue
			}
		}
	}
}

func (r *Runner) insertInbox(ctx context.Context, rec *kgo.Record) (int64, bool, error) {
	if rec == nil {
		return 0, false, errors.New("nil record")
	}
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO payments_inbox (topic, partition, "offset", key, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (topic, partition, "offset") DO NOTHING
		RETURNING id;
	`, rec.Topic, rec.Partition, rec.Offset, rec.Key, rec.Value).Scan(&id)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("insert payments_inbox: %w", err)
	}
	return id, true, nil
}

func (r *Runner) markProcessed(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE payments_inbox SET processed_at = now() WHERE id = $1 AND processed_at IS NULL;`, id)
	return err
}
