package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	HeaderRequestID = "X-Request-ID"
	CtxKeyReqID     = "req_id"
	CtxKeyLogger    = "req_logger"
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
				attrs := []slog.Attr{
					slog.Any("panic", rec),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
				}
				if rid, ok := c.Get(CtxKeyReqID); ok {
					if s, _ := rid.(string); s != "" {
						attrs = append(attrs, slog.String("req_id", s))
					}
				}
				log.LogAttrs(c.Request.Context(), slog.LevelError, "panic recovered", attrs...)
				debug.PrintStack()
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func ginLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		rid, _ := c.Get(CtxKeyReqID)
		reqLog := log.With(
			slog.String("method", c.Request.Method),
			slog.String("path", c.FullPath()),
		)
		if s, _ := rid.(string); s != "" {
			reqLog = reqLog.With(slog.String("req_id", s))
		}
		c.Set(CtxKeyLogger, reqLog)

		c.Next()

		lat := time.Since(start)
		status := c.Writer.Status()
		bytes := c.Writer.Size()

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.Int64("dur_ms", lat.Milliseconds()),
			slog.Int("bytes", bytes),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.String("query", c.Request.URL.RawQuery),
		}

		switch {
		case status >= 500:
			reqLog.LogAttrs(c.Request.Context(), slog.LevelError, "http", attrs...)
		case status >= 400:
			reqLog.LogAttrs(c.Request.Context(), slog.LevelWarn, "http", attrs...)
		default:
			reqLog.LogAttrs(c.Request.Context(), slog.LevelInfo, "http", attrs...)
		}
	}
}

func newReqID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
