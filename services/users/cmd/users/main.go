package main

import (
	"context"
	"errors"
	"fmt"
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

func main() {
	start := time.Now()

	// OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config & Logger
	cfg := config.New()
	log := logger.NewPrettyLogger(cfg.Logger)

	log.Info("boot: starting users service",
		"app", cfg.AppName,
		"http_addr", cfg.HTTP.Addr,
	)
	log.Debug("boot: config (redacted)", "cfg", cfg.Redact())

	// Postgres
	pgStart := time.Now()
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Error("postgres: connect failed", "host", cfg.Postgres.Host, "port", cfg.Postgres.Port, "db", cfg.Postgres.DBName, "err", err)
		return
	}
	log.Info("postgres: connected",
		"dsn", fmt.Sprintf("%s@%s:%d/%s", cfg.Postgres.User, cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.DBName),
		"latency", time.Since(pgStart),
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

	// HTTP module
	usersHTTP := httpmod.NewModule(log, pool, svc, jwtm)

	// HTTP server with modules
	srv := httpx.NewServer(cfg.HTTP, log,
		httpx.WithModules(usersHTTP),
	)

	// HTTP Listen
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http: listen failed", "err", err)
			stop()
		}
	}()

	// Wait for signal
	<-ctx.Done()
	log.Info("shutdown: received signal, stopping...")

	// graceful HTTP
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http: graceful shutdown failed", "err", err)
	} else {
		log.Info("http: server stopped cleanly")
	}
	log.Info("bye", "uptime", time.Since(start), "pid", os.Getpid())
}
