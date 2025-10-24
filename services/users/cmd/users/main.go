package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goshop/pkg/httpx"
	"goshop/pkg/jwtauth"
	"goshop/pkg/logger"
	"goshop/pkg/postgres"
	"goshop/services/users/config"
	httpmod "goshop/services/users/internal/adapters/http"
	"goshop/services/users/internal/adapters/repo/userpg"
	"goshop/services/users/internal/app"
)

const shutdownTimeout = 5 * time.Second

func main() {
	start := time.Now()

	// OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config & Logger
	cfg := config.New()
	log := logger.NewLogger(cfg.Logger)
	slog.SetDefault(log)

	if err := cfg.Validate(); err != nil {
		log.Error("config: invalid", slog.Any("err", err))
		return
	}

	log.Info("users: starting",
		slog.String("http.addr", cfg.HTTP.Addr),
	)
	log.Debug("users: config (redacted)", slog.Any("cfg", cfg.Redact()))

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

	// Domain wiring
	repo := userpg.NewRepo(pool)
	svc := app.NewService(repo, 12)

	// JWT
	jwtm := jwtauth.New(jwtauth.Config{
		Secret:          cfg.JWT.Secret,
		Issuer:          cfg.JWT.Issuer,
		AccessTTL:       cfg.JWT.AccessTTL,
		RefreshTTL:      cfg.JWT.RefreshTTL,
		AccessAudience:  cfg.JWT.AccessAudience,
		RefreshAudience: cfg.JWT.RefreshAudience,
	})
	log.Info("jwt: verifier initialized", "issuer", cfg.JWT.Issuer)

	// HTTP module + server
	usersHTTP := httpmod.NewModule(log, pool, svc, jwtm)
	srv := httpx.NewServer(cfg.HTTP, log, httpx.WithModules(usersHTTP))

	// HTTP Listen
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http: listen failed", slog.Any("err", err))
			stop()
		}
	}()

	// Wait for signal
	<-ctx.Done()
	log.Info("users: shutdown: signal received")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http: graceful shutdown failed", slog.Any("err", err))
	} else {
		log.Info("http: server stopped cleanly")
	}

	log.Info("bye",
		slog.Int("pid", os.Getpid()),
		slog.Int64("uptime_ms", time.Since(start).Milliseconds()),
	)
}
