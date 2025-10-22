// services/gateway/cmd/gateway/main.go
package main

import (
	"context"
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

	opts := server.Options{
		Addr:           cfg.GRPC.Addr,
		OrdersGRPCAddr: cfg.OrdersGRPC.Addr,
		OrdersTimeout:  cfg.OrdersGRPC.Timeout,
		Logger:         log,
		EnableReflect:  true,
	}

	if err := server.Start(ctx, opts); err != nil {
		log.Error("gateway exited with error", "err", err)
	}
}
