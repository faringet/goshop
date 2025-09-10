package main

import (
	"context"
	"errors"
	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/users/config"
	httpserver "goshop/services/users/internal/adapters/http"
	"goshop/services/users/internal/adapters/repo/userpg"
	"goshop/services/users/internal/app"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config
	cfg := config.NewConfig()

	// Logger
	log := logger.NewPrettyLogger(cfg.Logger)

	// Postgres
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Error("postgres: connect failed", "host", cfg.Postgres.Host, "port", cfg.Postgres.Port, "db", cfg.Postgres.DBName, "err", err)
		return
	}
	defer pool.Close()

	repo := userpg.NewRepo(pool)
	svc := app.NewService(repo, 12)

	// Server
	srv := httpserver.NewBuilder(cfg.HTTP, log).WithDB(pool).WithDefaultEndpoints().WithUsers(svc).Build()

	// Graceful shutdown (Ctrl+C / SIGTERM)
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
