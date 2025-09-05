package main

import (
	"context"
	"errors"
	"goshop/pkg/logger"
	"log/slog"
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

	// 3) HTTP-mux с базовыми health-пробами.
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", okHandler)
	mux.HandleFunc("/live", okHandler)
	mux.HandleFunc("/ready", okHandler)

	// (опционально можно подключить pprof позже)
	// import _ "net/http/pprof" и добавить:
	// mux.HandleFunc("/debug/pprof/", pprof.Index) ... — сделаем отдельным шагом

	// 4) HTTP-сервер с таймаутами из конфига.
	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           mux,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 5) Грейсфул-лайфцикл.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("http: listening", slog.String("addr", cfg.HTTP.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http: listen failed", slog.String("err", err.Error()))
			stop() // триггерим shutdown
		}
	}()

	<-ctx.Done()
	log.Info("shutdown: received signal, stopping...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http: graceful shutdown failed", slog.String("err", err.Error()))
	} else {
		log.Info("http: server stopped cleanly")
	}
}

// okHandler — простой 200 OK.
func okHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
