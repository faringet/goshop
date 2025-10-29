package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

const (
	readyPingTimeout = 500 * time.Millisecond
	dbPingTimeout    = 300 * time.Millisecond
)

type HealthHandlers struct {
	log *slog.Logger
	db  *pgxpool.Pool
}

func NewHealthHandlers(log *slog.Logger, db *pgxpool.Pool) *HealthHandlers {
	return &HealthHandlers{log: log, db: db}
}

func (h *HealthHandlers) Live(c *gin.Context) {
	noCache(c)
	c.String(http.StatusOK, "ok")
}

func (h *HealthHandlers) Ready(c *gin.Context) {
	noCache(c)

	l := reqLog(c, h.log)

	if h.db == nil {
		l.Error("orders.health.ready: db pool is nil",
			slog.String("path", c.FullPath()))
		c.String(http.StatusServiceUnavailable, "db not ready")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), readyPingTimeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		l.Error("orders.health.ready: db ping failed",
			slog.String("path", c.FullPath()),
			slog.Any("err", err))
		c.String(http.StatusServiceUnavailable, "db not ready")
		return
	}

	c.String(http.StatusOK, "ok")
}

func (h *HealthHandlers) DBPing(c *gin.Context) {
	noCache(c)

	l := reqLog(c, h.log)

	if h.db == nil {
		l.Error("orders.health.dbping: db pool is nil",
			slog.String("path", c.FullPath()))
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "fail", "err": "db is nil"})
		return
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(c.Request.Context(), dbPingTimeout)
	defer cancel()

	var one int
	if err := h.db.QueryRow(ctx, "select 1").Scan(&one); err != nil || one != 1 {
		l.Error("orders.health.dbping: query failed",
			slog.String("path", c.FullPath()),
			slog.Any("err", err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "fail", "err": "db query failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"latency_ms": time.Since(start).Milliseconds(),
	})
}
