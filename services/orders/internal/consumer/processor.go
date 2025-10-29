package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
)

type paymentEvent struct {
	Event     string    `json:"event"` // "payment.confirmed" | "payment.failed"
	Version   int       `json:"version"`
	PaymentID uuid.UUID `json:"payment_id"`
	OrderID   uuid.UUID `json:"order_id"`
	UserID    uuid.UUID `json:"user_id"`
	Amount    int64     `json:"amount_cents"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"` // "confirmed" | "failed"
	Reason    *string   `json:"reason,omitempty"`
}

type Processor struct {
	log *slog.Logger
	db  *pgxpool.Pool
	rds *redis.Client
	ttl time.Duration
}

func NewProcessor(log *slog.Logger, db *pgxpool.Pool, rds *redis.Client, ttl time.Duration) *Processor {
	return &Processor{log: log, db: db, rds: rds, ttl: ttl}
}

func (p *Processor) ProcessRecord(ctx context.Context, rec *kgo.Record) error {
	var meta struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(rec.Value, &meta); err != nil {
		return nil
	}

	switch meta.Event {
	case "payment.confirmed", "payment.failed":
		var ev paymentEvent
		if err := json.Unmarshal(rec.Value, &ev); err != nil {
			p.log.Warn("orders.processor: bad payment event payload",
				slog.String("topic", rec.Topic),
				slog.Int64("partition", int64(rec.Partition)),
				slog.Int64("offset", rec.Offset),
				slog.Any("err", err),
			)
			return nil
		}
		return p.applyPayment(ctx, ev)

	default:
		return nil
	}
}

func (p *Processor) applyPayment(ctx context.Context, ev paymentEvent) error {
	var want string
	switch ev.Event {
	case "payment.confirmed":
		want = "paid"
	case "payment.failed":
		want = "cancelled"
	default:
		return nil
	}

	const qUpdate = `
		UPDATE orders
		SET status = $2, updated_at = now()
		WHERE id = $1 AND status <> $2
	`
	tag, err := p.db.Exec(ctx, qUpdate, ev.OrderID, want)
	if err != nil {
		return fmt.Errorf("update orders status: %w", err)
	}

	if tag.RowsAffected() > 0 {
		p.setStatusCache(ctx, ev.OrderID.String(), want)
		p.log.Info("orders.processor: status updated",
			slog.String("order_id", ev.OrderID.String()),
			slog.String("to", want),
			slog.String("event", ev.Event),
		)
		return nil
	}

	var cur string
	if err := p.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, ev.OrderID).Scan(&cur); err != nil {
		p.log.Warn("orders.processor: read current status after noop failed",
			slog.String("order_id", ev.OrderID.String()),
			slog.Any("err", err),
		)
		return nil
	}

	p.setStatusCache(ctx, ev.OrderID.String(), cur)
	p.log.Info("orders.processor: payment applied (noop)",
		slog.String("order_id", ev.OrderID.String()),
		slog.String("kept", cur),
		slog.String("event", ev.Event),
	)
	return nil
}

func (p *Processor) setStatusCache(ctx context.Context, orderID, status string) {
	if p.rds == nil {
		return
	}
	key := "order:" + orderID + ":status"
	if err := p.rds.Set(ctx, key, status, p.ttl).Err(); err != nil {
		p.log.Warn("orders.processor.redis: set status failed",
			slog.String("key", key),
			slog.String("status", status),
			slog.Any("err", err),
		)
		return
	}
	p.log.Debug("orders.processor: status cached",
		slog.String("key", key),
		slog.String("status", status),
		slog.Int64("ttl_ms", p.ttl.Milliseconds()),
	)
}
