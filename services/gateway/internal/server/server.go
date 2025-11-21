package server

import (
	"context"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"net"
	"time"

	"log/slog"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

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
	log := opt.Logger
	if log == nil {
		log = slog.Default()
	}

	lis, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		log.Error("gateway.server: listen failed",
			slog.String("addr", opt.Addr),
			slog.Any("err", err),
		)
		return err
	}

	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			selector.UnaryServerInterceptor(
				logging.UnaryServerInterceptor(
					slogLogger(log),
					logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
				),
				selector.MatchFunc(skipHealthLogs),
			),
			recovery.UnaryServerInterceptor(),
		),
	}
	s := grpc.NewServer(serverOpts...)

	// health
	hs := health.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// checkout service
	svc, err := service.NewCheckoutService(ctx, service.Options{
		OrdersAddr:  opt.OrdersGRPCAddr,
		OrdersTO:    opt.OrdersTimeout,
		Logger:      log,
		DefaultCurr: "RUB",
		Redis:       opt.Redis,
	})
	if err != nil {
		log.Error("gateway.server: checkout init failed", slog.Any("err", err))
		_ = lis.Close()
		return err
	}
	checkoutpb.RegisterCheckoutServer(s, svc)

	if opt.EnableReflect {
		reflection.Register(s)
	}

	log.Info("gateway.server: listening",
		slog.String("addr", opt.Addr),
		slog.String("orders_grpc_addr", opt.OrdersGRPCAddr),
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
		log.Info("gateway.server: shutting down", slog.String("reason", "context done"))
		s.GracefulStop()
		return nil
	case err := <-serveErrCh:
		if err != nil {
			log.Error("gateway.server: serve stopped with error", slog.Any("err", err))
			return err
		}
		return nil
	}
}

func skipHealthLogs(_ context.Context, c interceptors.CallMeta) bool {
	return c.Service != "grpc.health.v1.Health"
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
