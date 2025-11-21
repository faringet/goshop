package server

import (
	"context"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"net"

	"log/slog"

	"google.golang.org/grpc"

	"goshop/services/opsassistant/api/opspb"
)

type Options struct {
	Addr string
	Svc  opspb.OpsAssistantServer
	Log  *slog.Logger
}

func Start(ctx context.Context, opt Options) error {
	log := opt.Log
	if log == nil {
		log = slog.Default()
	}

	lis, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		log.Error("opsassistant.server: listen failed",
			slog.String("addr", opt.Addr),
			slog.Any("err", err),
		)
		return err
	}

	s := grpc.NewServer()
	opspb.RegisterOpsAssistantServer(s, opt.Svc)

	// health
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, hs)

	errCh := make(chan error, 1)
	go func() {
		log.Info("opsassistant.server: listening", slog.String("addr", opt.Addr))
		errCh <- s.Serve(lis)
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info("opsassistant.server: shutting down", slog.String("reason", "context done"))
		hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		s.GracefulStop()
		log.Info("opsassistant.server: stopped")
		return nil
	case err := <-errCh:
		if err != nil {
			log.Error("opsassistant.server: serve stopped with error", slog.Any("err", err))
			return err
		}
		log.Info("opsassistant.server: stopped")
		return nil
	}
}
