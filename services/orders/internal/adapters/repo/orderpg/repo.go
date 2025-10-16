package orderpg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type Order struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Status      string
	TotalAmount float64
	Currency    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateParams struct {
	UserID        uuid.UUID
	AmountCents   int64
	Currency      string
	OutboxTopic   string
	OutboxHeaders map[string]string
}

func (r *Repository) Create(ctx context.Context, p CreateParams) (*Order, error) {
	if p.UserID == uuid.Nil {
		return nil, errors.New("user_id is required")
	}
	if p.AmountCents <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	if p.Currency == "" {
		p.Currency = "RUB"
	}
	if p.OutboxTopic == "" {
		p.OutboxTopic = "orders.events"
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insOrder = `
		INSERT INTO orders (user_id, status, total_amount, currency)
		VALUES ($1, 'new', $2/100.0, $3)
		RETURNING id, user_id, status, total_amount, currency, created_at, updated_at;
	`

	var ord Order
	if err := tx.QueryRow(ctx, insOrder,
		p.UserID, p.AmountCents, p.Currency,
	).Scan(&ord.ID, &ord.UserID, &ord.Status, &ord.TotalAmount, &ord.Currency, &ord.CreatedAt, &ord.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}

	type OrderCreated struct {
		Event     string    `json:"event"`
		Version   int       `json:"version"`
		OrderID   uuid.UUID `json:"order_id"`
		UserID    uuid.UUID `json:"user_id"`
		Amount    int64     `json:"amount_cents"`
		Currency  string    `json:"currency"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	}
	payload := OrderCreated{
		Event:     "order.created",
		Version:   1,
		OrderID:   ord.ID,
		UserID:    ord.UserID,
		Amount:    p.AmountCents,
		Currency:  p.Currency,
		Status:    ord.Status,
		CreatedAt: ord.CreatedAt.UTC(),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal outbox payload: %w", err)
	}

	type hdr struct{ K, V string }
	var headersJSON []byte
	if len(p.OutboxHeaders) > 0 {
		hs := make([]hdr, 0, len(p.OutboxHeaders))
		for k, v := range p.OutboxHeaders {
			hs = append(hs, hdr{K: k, V: v})
		}
		headersJSON, err = json.Marshal(hs)
		if err != nil {
			return nil, fmt.Errorf("marshal headers: %w", err)
		}
	} else {
		headersJSON = []byte("[]")
	}

	const insOutbox = `
		INSERT INTO orders_outbox (agg_type, agg_id, topic, key, headers, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb);
	`
	key := ord.ID[:]

	if _, err := tx.Exec(ctx, insOutbox,
		"order", ord.ID, p.OutboxTopic, key, headersJSON, payloadJSON,
	); err != nil {
		return nil, fmt.Errorf("insert outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &ord, nil
}
