// services/gateway/cmd/gateway/main.go
package main

import (
	"context"
	"github.com/redis/go-redis/v9"
	"os/signal"
	"syscall"

	"log/slog"

	"goshop/services/gateway/config"
	"goshop/services/gateway/internal/server"
)

func main() {
	// OS signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Config & Logger
	cfg := config.New()
	log := slog.Default()

	log.Info("gateway: starting",
		"grpc_addr", cfg.GRPC.Addr,
		"orders_grpc_addr", cfg.OrdersGRPC.Addr,
	)

	// NEW: Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Error("gateway: redis ping failed", "addr", cfg.Redis.Addr, "err", err)
		return
	}
	log.Info("gateway: redis connected", "addr", cfg.Redis.Addr)

	// Server
	opts := server.Options{
		Addr:           cfg.GRPC.Addr,
		OrdersGRPCAddr: cfg.OrdersGRPC.Addr,
		OrdersTimeout:  cfg.OrdersGRPC.Timeout,
		Logger:         log,
		EnableReflect:  true,
		Redis:          rdb,
	}

	if err := server.Start(ctx, opts); err != nil {
		log.Error("gateway exited with error", "err", err)
	}
}
