package main

import (
	"context"
	"errors"
	"goshop/pkg/logger"
	httpserver "goshop/services/users/internal/adapters/http"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	svcconfig "goshop/services/users/internal/config"
)

func main() {
	// 1) Загрузка конфигурации сервиса (defaults.yaml → users.yaml → ENV USERS_*).
	cfg := svcconfig.MustLoad()

	// 2) Логгер
	//log := logger.New(cfg.Logger)
	log := logger.NewPretty(cfg.Logger)

	// 3) HTTP-сервер (Gin) со здоровьем и middleware
	srv := httpserver.New(cfg.HTTP, log)

	// 4) Грейсфул-лайфцикл: Ctrl+C / SIGTERM → аккуратная остановка
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http: listen failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutdown: received signal, stopping...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http: graceful shutdown failed", "err", err)
	} else {
		log.Info("http: server stopped cleanly")
	}
}
