package grpcsvr

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"goshop/services/orders/api/orderspb"
	"goshop/services/orders/internal/adapters/repo/orderpg"
)

type Options struct {
	Addr   string
	Logger *slog.Logger
	Repo   *orderpg.Repository
}

type Server struct {
	orderspb.UnimplementedOrdersServer
	log  *slog.Logger
	repo *orderpg.Repository
}

func (s *Server) CreateOrder(ctx context.Context, in *orderspb.CreateOrderRequest) (*orderspb.CreateOrderResponse, error) {
	if in == nil || in.UserId == "" || in.AmountCents <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and positive amount_cents are required")
	}

	uid, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "bad user_id")
	}
	curr := in.Currency
	if curr == "" {
		curr = "RUB"
	}

	ord, err := s.repo.Create(ctx, orderpg.CreateParams{
		UserID:        uid,
		AmountCents:   in.AmountCents,
		Currency:      curr,
		OutboxTopic:   "orders.events",
		OutboxHeaders: map[string]string{"event-type": "order.created", "source": "orders-grpc"},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create order: %v", err)
	}

	resp := &orderspb.CreateOrderResponse{
		OrderId:     ord.ID.String(),
		Status:      toPbStatus(ord.Status),
		Currency:    ord.Currency,
		TotalAmount: ord.TotalAmount,
		CreatedAt:   ord.CreatedAt.UTC().Format(time.RFC3339),
	}
	return resp, nil
}

func Start(ctx context.Context, opt Options) error {
	if opt.Logger == nil {
		opt.Logger = slog.Default()
	}
	lis, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		opt.Logger.Error("orders.grpc: listen failed",
			slog.String("addr", opt.Addr),
			slog.Any("err", err),
		)
		return err
	}

	s := grpc.NewServer()
	orderspb.RegisterOrdersServer(s, &Server{log: opt.Logger, repo: opt.Repo})

	errCh := make(chan error, 1)
	go func() {
		opt.Logger.Info("orders.grpc: listening", slog.String("addr", opt.Addr))
		errCh <- s.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		opt.Logger.Info("orders.grpc: shutting down", slog.String("reason", "context done"))
		s.GracefulStop()
		return nil
	case err := <-errCh:
		opt.Logger.Error("orders.grpc: serve stopped with error", slog.Any("err", err))
		return err
	}
}

func toPbStatus(s string) orderspb.OrderStatus {
	switch s {
	case "new":
		return orderspb.OrderStatus_ORDER_STATUS_NEW
	case "paid":
		return orderspb.OrderStatus_ORDER_STATUS_PAID
	case "cancelled", "canceled":
		return orderspb.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return orderspb.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}
