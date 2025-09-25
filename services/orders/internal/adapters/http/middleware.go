package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	HeaderRequestID = "X-Request-ID"
	CtxKeyReqID     = "req_id"
)

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(HeaderRequestID)
		if rid == "" {
			rid = newReqID()
		}
		c.Writer.Header().Set(HeaderRequestID, rid)
		c.Set(CtxKeyReqID, rid)
		c.Next()
	}
}

func ginRecovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered",
					slog.Any("panic", rec),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func ginLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		lat := time.Since(start)
		status := c.Writer.Status()
		bytes := c.Writer.Size()
		rid, _ := c.Get(CtxKeyReqID)

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.FullPath()),
			slog.Int("status", status),
			slog.Duration("dur", lat),
			slog.Int("bytes", bytes),
		}
		if ridStr, ok := rid.(string); ok && ridStr != "" {
			attrs = append(attrs, slog.String("req_id", ridStr))
		}

		switch {
		case status >= 500:
			log.LogAttrs(c.Request.Context(), slog.LevelError, "http", attrs...)
		case status >= 400:
			log.LogAttrs(c.Request.Context(), slog.LevelWarn, "http", attrs...)
		default:
			log.LogAttrs(c.Request.Context(), slog.LevelInfo, "http", attrs...)
		}
	}
}

func newReqID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
