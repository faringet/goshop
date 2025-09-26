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
	r.log.Info("orders-consumer: starting",
		"group", r.cfg.Group,
		"topic", r.cfg.Topic,
	)
	defer close(r.done)

	for {
		select {
		case <-ctx.Done():
			r.log.Info("orders-consumer: stopping (context done)")
			return ctx.Err()
		default:
		}

		fetches := r.kc.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				r.log.Warn("kafka fetch error",
					"topic", fe.Topic,
					"partition", fe.Partition,
					"err", fe.Err,
				)
			}
			continue
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()

			inserted, err := r.insertInbox(ctx, rec)
			if err != nil {
				r.log.Error("inbox insert failed", "topic", rec.Topic, "partition", rec.Partition, "offset", rec.Offset, "err", err)
				continue
			}
			if !inserted {
				continue
			}

			r.log.Info("inbox ok",
				"topic", rec.Topic,
				"partition", rec.Partition,
				"offset", rec.Offset,
			)

			if r.proc != nil {
				if err := r.proc.ProcessRecord(ctx, rec); err != nil {
					r.log.Error("processor error",
						"topic", rec.Topic,
						"partition", rec.Partition,
						"offset", rec.Offset,
						"err", err,
					)
					// не падаем, след запись
				}
			}
		}
	}
}

func (r *Runner) insertInbox(ctx context.Context, rec *kgo.Record) (bool, error) {
	if rec == nil {
		return false, errors.New("nil record")
	}
	tag, err := r.db.Exec(ctx, `
		INSERT INTO inbox (topic, partition, "offset", key, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (topic, partition, "offset") DO NOTHING;
	`,
		rec.Topic, rec.Partition, rec.Offset, rec.Key, rec.Value,
	)
	if err != nil {
		return false, fmt.Errorf("exec insert inbox: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
