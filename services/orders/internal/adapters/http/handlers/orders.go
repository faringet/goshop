package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"goshop/services/orders/internal/adapters/repo/orderpg"
)

type OrdersHandlers struct {
	log  *slog.Logger
	repo *orderpg.Repository
}

func NewOrdersHandlers(log *slog.Logger, repo *orderpg.Repository) *OrdersHandlers {
	return &OrdersHandlers{log: log, repo: repo}
}

type createOrderReq struct {
	UserID      string `json:"user_id"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}
type createOrderResp struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
	Currency    string  `json:"currency"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func (h *OrdersHandlers) Create(c *gin.Context) {
	noCache(c)

	var in createOrderReq
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if in.UserID == "" || in.AmountCents <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and positive amount_cents are required"})
		return
	}
	uid, err := uuid.Parse(in.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	ord, err := h.repo.Create(ctx, orderpg.CreateParams{
		UserID:      uid,
		AmountCents: in.AmountCents,
		Currency:    in.Currency,
		OutboxTopic: "orders.events",
		OutboxHeaders: map[string]string{
			"event-type": "order.created",
			"source":     "orders-http",
		},
	})
	if err != nil {
		h.log.Error("orders.create failed", slog.Any("err", err))
		if errors.Is(err, &json.SyntaxError{}) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad payload"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusCreated)
	_ = json.NewEncoder(c.Writer).Encode(createOrderResp{
		ID:          ord.ID.String(),
		UserID:      ord.UserID.String(),
		Status:      ord.Status,
		TotalAmount: ord.TotalAmount,
		Currency:    ord.Currency,
		CreatedAt:   ord.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   ord.UpdatedAt.UTC().Format(time.RFC3339),
	})
}
