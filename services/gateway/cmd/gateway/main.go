package main

import (
	"context"
	"goshop/pkg/metrics"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"goshop/pkg/logger"
	"goshop/services/gateway/config"
	"goshop/services/gateway/internal/server"
)

func main() {
	start := time.Now()

	// OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config + Logger (
	cfg := config.New()
	log := logger.NewLogger(cfg.Logger)
	slog.SetDefault(log)

	log.Info("gateway: starting",
		slog.String("grpc.addr", cfg.GRPC.Addr),
		slog.String("orders.grpc.addr", cfg.OrdersGRPC.Addr),
		slog.String("redis.addr", cfg.Redis.Addr),
	)

	// ── Metrics server (Prometheus) ────────────────────────────────────────────
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "docker"
	}
	met, err := metrics.Init(log, metrics.Config{
		Service:   "gateway",
		Namespace: "goshop",
		Addr:      ":2112",
		Version:   "dev",
		Env:       env,
	})
	if err != nil {
		log.Error("metrics: init failed", slog.Any("err", err))
		return
	}
	defer func() { _ = met.Shutdown(context.Background()) }()

	// gRPC metrics (с быстрыми веб-бакетами)
	grpcm := metrics.NewGRPCMetrics(
		met.Registry(),
		"goshop",
		"gateway",
		metrics.WithGRPCBuckets(metrics.WebFastBuckets),
	)

	// Redis client
	rdStart := time.Now()
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Error("gateway: redis ping failed",
			slog.String("addr", cfg.Redis.Addr),
			slog.Any("err", err),
		)
		return
	}
	log.Info("gateway: redis connected",
		slog.String("addr", cfg.Redis.Addr),
		slog.Int64("latency_ms", time.Since(rdStart).Milliseconds()),
	)
	defer func() { _ = rdb.Close() }()

	// Server
	opts := server.Options{
		Addr:           cfg.GRPC.Addr,
		OrdersGRPCAddr: cfg.OrdersGRPC.Addr,
		OrdersTimeout:  cfg.OrdersGRPC.Timeout,
		Logger:         log,
		EnableReflect:  true,
		Redis:          rdb,

		Unary:  []server.UnaryInt{grpcm.UnaryServerInterceptor()},
		Stream: []server.StreamInt{grpcm.StreamServerInterceptor()},
	}

	if err := server.Start(ctx, opts); err != nil {
		log.Error("gateway: exited with error", slog.Any("err", err))
		return
	}

	log.Info("gateway: stopped",
		slog.Int64("uptime_ms", time.Since(start).Milliseconds()),
	)
}
