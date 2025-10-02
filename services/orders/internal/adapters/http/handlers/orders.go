package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"goshop/pkg/httpx"
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

	l := reqLog(c, h.log)

	claims, ok := httpx.GetJWTClaims(c)
	if !ok || claims.UserID == "" {
		l.Warn("create: no jwt claims in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		l.Warn("create: invalid uid in jwt", "uid", claims.UserID)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var in createOrderReq
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if in.AmountCents <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_cents must be > 0"})
		return
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if cur == "" {
		cur = "RUB"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	ord, err := h.repo.Create(ctx, orderpg.CreateParams{
		UserID:      userUUID,
		AmountCents: in.AmountCents,
		Currency:    cur,
		OutboxTopic: "orders.events",
		OutboxHeaders: map[string]string{
			"event-type": "order.created",
			"source":     "orders-http",
		},
	})
	if err != nil {
		l.Error("orders.create failed", slog.Any("err", err))
		var se *json.SyntaxError
		if errors.As(err, &se) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad payload"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Cache-Control", "no-store")
	c.Header("Location", fmt.Sprintf("/v1/orders/%s", ord.ID.String()))
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

func noCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}

func reqLog(c *gin.Context, fallback *slog.Logger) *slog.Logger {
	if rl, ok := c.Get(httpx.CtxKeyLogger); ok {
		if l, ok := rl.(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return fallback
}
