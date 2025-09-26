package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
}

func NewProcessor(log *slog.Logger, db *pgxpool.Pool) *Processor {
	return &Processor{log: log, db: db}
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
			p.log.Warn("orders: bad payment event payload", "err", err)
			return nil
		}
		return p.applyPayment(ctx, ev)

	default:
		return nil
	}
}

func (p *Processor) applyPayment(ctx context.Context, ev paymentEvent) error {
	var newStatus string
	switch ev.Event {
	case "payment.confirmed":
		newStatus = "paid"
	case "payment.failed":
		newStatus = "cancelled"
	default:
		return nil
	}

	cmd := `
		UPDATE orders
		SET status = $2, updated_at = now()
		WHERE id = $1 AND status = 'new'
	`
	tag, err := p.db.Exec(ctx, cmd, ev.OrderID, newStatus)
	if err != nil {
		return fmt.Errorf("update orders status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		p.log.Info("orders: payment event applied (noop)",
			"order_id", ev.OrderID, "to", newStatus)
		return nil
	}

	p.log.Info("orders: status updated",
		"order_id", ev.OrderID, "to", newStatus)
	return nil
}
