package server

import (
	"context"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"goshop/services/opsassistant/api/opspb"
)

type Options struct {
	Addr string
	Svc  opspb.OpsAssistantServer
	Log  *slog.Logger
}

func Start(ctx context.Context, opt Options) error {
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	lis, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		return err
	}

	s := grpc.NewServer()
	opspb.RegisterOpsAssistantServer(s, opt.Svc)

	errCh := make(chan error, 1)
	go func() {
		opt.Log.Info("ops-assistant: listening", "addr", opt.Addr)
		errCh <- s.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		s.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}
