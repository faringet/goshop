package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

type Config struct {
	OutboxTable    string
	BatchSize      int
	PollInterval   time.Duration
	ProduceTimeout time.Duration
	MaxRetries     int
	BackoffBaseMS  int
}

type Worker struct {
	db  *pgxpool.Pool
	kc  *kgo.Client
	cfg Config
	tbl string
}

func New(db *pgxpool.Pool, kc *kgo.Client, cfg Config) *Worker {
	if cfg.OutboxTable == "" {
		cfg.OutboxTable = "outbox"
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.ProduceTimeout <= 0 {
		cfg.ProduceTimeout = 3 * time.Second
	}
	if cfg.BackoffBaseMS <= 0 {
		cfg.BackoffBaseMS = 500
	}
	return &Worker{db: db, kc: kc, cfg: cfg, tbl: cfg.OutboxTable}
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if err := w.processBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) error {
	tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sel := fmt.Sprintf(`
SELECT id, topic, key, headers, payload, retries
FROM %s
WHERE published_at IS NULL
  AND (available_at IS NULL OR available_at <= now())
ORDER BY id
FOR UPDATE SKIP LOCKED
LIMIT $1;`, w.tbl)

	rows, err := tx.Query(ctx, sel, w.cfg.BatchSize)
	if err != nil {
		return fmt.Errorf("select %s: %w", w.tbl, err)
	}
	defer rows.Close()

	type item struct {
		id      int64
		topic   string
		key     []byte
		headers []byte
		payload []byte
		retries int
	}
	var batch []item

	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.topic, &it.key, &it.headers, &it.payload, &it.retries); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		batch = append(batch, it)
	}
	if rows.Err() != nil {
		return fmt.Errorf("rows: %w", rows.Err())
	}

	if len(batch) == 0 {
		return nil
	}

	records := make([]*kgo.Record, 0, len(batch))
	type hdr struct{ K, V string }
	for _, it := range batch {
		var hs []hdr
		if len(it.headers) > 0 {
			_ = json.Unmarshal(it.headers, &hs)
		}
		khs := make([]kgo.RecordHeader, 0, len(hs))
		for _, h := range hs {
			khs = append(khs, kgo.RecordHeader{Key: h.K, Value: []byte(h.V)})
		}
		records = append(records, &kgo.Record{
			Topic:   it.topic,
			Key:     it.key,
			Value:   it.payload,
			Headers: khs,
		})
	}

	pctx, cancel := context.WithTimeout(ctx, w.cfg.ProduceTimeout)
	defer cancel()
	results := w.kc.ProduceSync(pctx, records...)
	for i, res := range results {
		it := batch[i]

		if res.Err != nil {
			backoff := time.Duration(w.cfg.BackoffBaseMS) * time.Millisecond
			for j := 0; j < it.retries; j++ {
				backoff *= 2
				if backoff > 5*time.Minute {
					backoff = 5 * time.Minute
					break
				}
			}

			fail := fmt.Sprintf(`
UPDATE %s
SET retries = retries + 1,
    error = $2,
    available_at = now() + $3 * interval '1 millisecond'
WHERE id = $1;`, w.tbl)

			if _, err := tx.Exec(ctx, fail, it.id, res.Err.Error(), backoff.Milliseconds()); err != nil {
				return fmt.Errorf("mark fail %s: %w", w.tbl, err)
			}
			continue
		}

		ok := fmt.Sprintf(`
UPDATE %s
SET published_at = now(),
    error = NULL
WHERE id = $1;`, w.tbl)

		if _, err := tx.Exec(ctx, ok, it.id); err != nil {
			return fmt.Errorf("mark ok %s: %w", w.tbl, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// PingKafka todo прикручу позднее
func (w *Worker) PingKafka(ctx context.Context) error {
	req := kmsg.NewMetadataRequest()
	_, err := req.RequestWith(ctx, w.kc)
	return err
}
