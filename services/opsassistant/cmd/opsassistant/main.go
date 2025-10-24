package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"goshop/pkg/postgres"
	"goshop/services/opsassistant/config"
	"goshop/services/opsassistant/internal/ollama"
	"goshop/services/opsassistant/internal/server"
	"goshop/services/opsassistant/internal/service"
	"goshop/services/opsassistant/internal/timeline"
)

func main() {
	// graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// config & logger
	cfg := config.New()
	log := slog.Default()

	// postgres
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Error("pg connect failed", "err", err)
		return
	}
	defer pool.Close()

	llm := ollama.New(cfg.Ollama.BaseURL, cfg.Ollama.Model, 0, cfg.Ollama.Timeout)
	tl := timeline.New(pool)

	// Тихий прогрев модели
	go func() {
		log.Info("ollama: warm-up starting", "model", cfg.Ollama.Model)
		wctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := llm.Chat(wctx, "You are an ops assistant.", "ping"); err != nil {
			log.Warn("ollama: warm-up failed", "err", err)
			return
		}
		log.Info("ollama: warm-up completed", "model", cfg.Ollama.Model)
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
		log.Error("ops-assistant failed", "err", err)
	}
}
