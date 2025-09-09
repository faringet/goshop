package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	pcfg "goshop/pkg/config"

	"github.com/gin-gonic/gin"
)

type Server struct {
	srv *http.Server
	log *slog.Logger
}

func New(cfg pcfg.HTTP, log *slog.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(requestID())
	r.Use(ginLogger(log))
	r.Use(ginRecovery(log))

	r.GET("/healthz", okHandler)
	r.GET("/live", okHandler)
	r.GET("/ready", okHandler)

	v1 := r.Group("/v1")
	{
		v1.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	}

	s := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return &Server{srv: s, log: log}
}

func (s *Server) ListenAndServe() error {
	s.log.Info("http: listening", slog.String("addr", s.srv.Addr))
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func okHandler(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = newReqID()
		}
		c.Writer.Header().Set("X-Request-ID", rid)
		c.Set("req_id", rid)
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
		start := time.Now()         // 1) фиксируем время начала обработки запроса
		c.Next()                    // 2) пропускаем запрос дальше по middleware/handler'ам
		lat := time.Since(start)    // 3) вычисляем латентность
		status := c.Writer.Status() // 4) HTTP-статус ответа
		bytes := c.Writer.Size()    // 5) количество байт в ответе
		rid, _ := c.Get("req_id")   // 6) достаём X-Request-ID, который положили в requestID()

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
	var b [12]byte // 24 hex chars
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
