package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Processor struct {
	log         *slog.Logger
	db          *pgxpool.Pool
	outboxTopic string
}

func NewProcessor(log *slog.Logger, db *pgxpool.Pool, outboxTopic string) *Processor {
	return &Processor{log: log, db: db, outboxTopic: outboxTopic}
}

// входящее событие из orders (как формируем в orders repo)
type orderCreated struct {
	Event     string    `json:"event"`
	Version   int       `json:"version"`
	OrderID   uuid.UUID `json:"order_id"`
	UserID    uuid.UUID `json:"user_id"`
	Amount    int64     `json:"amount_cents"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// исходящее событие из payments
type paymentEvent struct {
	Event       string    `json:"event"` // payment.confirmed | payment.failed
	Version     int       `json:"version"`
	PaymentID   uuid.UUID `json:"payment_id"`
	OrderID     uuid.UUID `json:"order_id"`
	UserID      uuid.UUID `json:"user_id"`
	Amount      int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"` // confirmed | failed
	ProcessedAt time.Time `json:"processed_at"`
	Reason      *string   `json:"reason,omitempty"`
}

func (p *Processor) ProcessRecord(ctx context.Context, rec *kgo.Record) error {
	var meta struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(rec.Value, &meta); err != nil {
		p.log.Warn("payments: skip non-json payload", "err", err)
		return nil
	}

	switch meta.Event {
	case "order.created":
		var oc orderCreated
		if err := json.Unmarshal(rec.Value, &oc); err != nil {
			p.log.Warn("payments: bad order.created payload", "err", err)
			return nil
		}
		return p.handleOrderCreated(ctx, oc)
	default:
		return nil
	}
}

// простая “логика эквайринга”: каждое 5-е число фейлим — для демонстрации
func (p *Processor) decideStatus(amountCents int64) (status string, reason *string) {
	if amountCents%5 == 0 {
		r := "insufficient_funds"
		return "failed", &r
	}
	return "confirmed", nil
}

// одна транзакция: INSERT payments + INSERT payments_outbox
func (p *Processor) handleOrderCreated(ctx context.Context, oc orderCreated) error {
	status, reason := p.decideStatus(oc.Amount)

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var paymentID uuid.UUID
	now := time.Now().UTC()

	// 1) запись в payments
	err = tx.QueryRow(ctx, `
		INSERT INTO payments (order_id, user_id, amount_cents, currency, status, provider, reason)
		VALUES ($1, $2, $3, $4, $5, 'mockpay', $6)
		RETURNING id;
	`, oc.OrderID, oc.UserID, oc.Amount, oc.Currency, status, reason).Scan(&paymentID)
	if err != nil {
		return fmt.Errorf("insert payments: %w", err)
	}

	// 2) публикация результата в payments_outbox (подберёт outboxer)
	ev := paymentEvent{
		Event:       "payment." + status,
		Version:     1,
		PaymentID:   paymentID,
		OrderID:     oc.OrderID,
		UserID:      oc.UserID,
		Amount:      oc.Amount,
		Currency:    oc.Currency,
		Status:      status,
		ProcessedAt: now,
		Reason:      reason,
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal payment event: %w", err)
	}

	// key — по order_id, чтобы downstream (orders) получал по тому же ключу
	key := oc.OrderID[:]

	_, err = tx.Exec(ctx, `
		INSERT INTO payments_outbox (agg_type, agg_id, topic, key, headers, payload)
		VALUES ('payment', $1, $2, $3, '[]'::jsonb, $4::jsonb);
	`, paymentID, p.outboxTopic, key, payload)
	if err != nil {
		return fmt.Errorf("insert payments_outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	p.log.Info("payments: processed", "order_id", oc.OrderID, "payment_id", paymentID, "status", status)
	return nil
}
