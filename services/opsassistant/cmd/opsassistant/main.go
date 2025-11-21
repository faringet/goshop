// services/opsassistant/cmd/opsassistant/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/opsassistant/config"
	"goshop/services/opsassistant/internal/ollama"
	"goshop/services/opsassistant/internal/server"
	"goshop/services/opsassistant/internal/service"
	"goshop/services/opsassistant/internal/timeline"
)

func main() {
	start := time.Now()

	// graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// config & logger
	cfg := config.New()
	log := logger.NewLogger(cfg.Logger)
	slog.SetDefault(log)

	if err := cfg.Validate(); err != nil {
		log.Error("config: invalid", slog.Any("err", err))
		return
	}

	log.Info("opsassistant: starting",
		slog.String("grpc.addr", cfg.GRPC.Addr),
		slog.String("ollama.model", cfg.Ollama.Model),
	)

	// Postgres
	pgStart := time.Now()
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Error("postgres: connect failed",
			slog.String("host", cfg.Postgres.Host),
			slog.Int("port", cfg.Postgres.Port),
			slog.String("db", cfg.Postgres.DBName),
			slog.Any("err", err),
		)
		return
	}
	log.Info("postgres: connected",
		slog.String("dsn", fmt.Sprintf("%s@%s:%d/%s", cfg.Postgres.User, cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.DBName)),
		slog.Int64("latency_ms", time.Since(pgStart).Milliseconds()),
	)
	defer pool.Close()

	// Ollama
	llm := ollama.New(cfg.Ollama.BaseURL, cfg.Ollama.Model, 0, cfg.Ollama.Timeout)
	tl := timeline.New(pool)

	// Тихий прогрев модели
	go func() {
		log.Info("opsassistant.ollama: warmup.start", slog.String("model", cfg.Ollama.Model))
		wctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := llm.Chat(wctx, "You are an ops assistant.", "ping"); err != nil {
			log.Warn("opsassistant.ollama: warmup.failed", slog.Any("err", err))
			return
		}
		log.Info("opsassistant.ollama: warmup.ok", slog.String("model", cfg.Ollama.Model))
	}()

	// service
	svc := service.New(service.Options{
		Logger: log,
		LLM:    llm,
		TL:     tl,
	})

	// server
	if err := server.Start(ctx, server.Options{
		Addr: cfg.GRPC.Addr,
		Svc:  svc,
		Log:  log,
	}); err != nil {
		log.Error("opsassistant: server failed", slog.Any("err", err))
		return
	}

	log.Info("opsassistant: stopped",
		slog.Int64("uptime_ms", time.Since(start).Milliseconds()),
	)
}
