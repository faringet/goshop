package server

import (
	"context"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"

	"goshop/services/gateway/api/checkoutpb"
	"goshop/services/gateway/internal/service"
)

type Options struct {
	Addr           string
	OrdersGRPCAddr string
	OrdersTimeout  time.Duration
	Logger         *slog.Logger
	EnableReflect  bool
	Redis          *redis.Client
}

func Start(ctx context.Context, opt Options) error {
	if opt.Logger == nil {
		opt.Logger = slog.Default()
	}

	lis, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		opt.Logger.Error("gateway: listen failed", "addr", opt.Addr, "err", err)
		return err
	}

	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(),
			logging.UnaryServerInterceptor(
				slogLogger(opt.Logger),
				logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
			),
		),
	}
	s := grpc.NewServer(serverOpts...)

	hs := health.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	svc, err := service.NewCheckoutService(ctx, service.Options{
		OrdersAddr:  opt.OrdersGRPCAddr,
		OrdersTO:    opt.OrdersTimeout,
		Logger:      opt.Logger,
		DefaultCurr: "RUB",
		Redis:       opt.Redis,
	})
	if err != nil {
		opt.Logger.Error("gateway: init checkout service failed", "err", err)
		_ = lis.Close()
		return err
	}
	checkoutpb.RegisterCheckoutServer(s, svc)

	if opt.EnableReflect {
		reflection.Register(s)
	}

	opt.Logger.Info("gateway: gRPC listening",
		"addr", opt.Addr,
		"orders_grpc_addr", opt.OrdersGRPCAddr,
	)

	serveErrCh := make(chan error, 1)
	go func() {
		if err := s.Serve(lis); err != nil {
			serveErrCh <- err
		}
		close(serveErrCh)
	}()

	select {
	case <-ctx.Done():
		opt.Logger.Info("gateway: shutting down...")
		s.GracefulStop()
		return nil
	case err := <-serveErrCh:
		if err != nil {
			opt.Logger.Error("gateway: serve failed", "err", err)
			return err
		}
		return nil
	}
}

func slogLogger(log *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		switch lvl {
		case logging.LevelDebug:
			log.Debug(msg, fields...)
		case logging.LevelInfo:
			log.Info(msg, fields...)
		case logging.LevelWarn:
			log.Warn(msg, fields...)
		case logging.LevelError:
			log.Error(msg, fields...)
		default:
			log.Info(msg, fields...)
		}
	})
}
