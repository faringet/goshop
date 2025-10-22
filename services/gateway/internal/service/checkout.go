// services/gateway/internal/service/checkout.go
package service

import (
	"context"
	"time"

	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"goshop/services/gateway/api/checkoutpb"
	"goshop/services/orders/api/orderspb"
)

type Options struct {
	OrdersAddr  string
	OrdersTO    time.Duration
	Logger      *slog.Logger
	DefaultCurr string
}

type CheckoutService struct {
	checkoutpb.UnimplementedCheckoutServer
	log         *slog.Logger
	orders      *OrdersGRPCClient
	defaultCurr string
}

func NewCheckoutService(ctx context.Context, opt Options) (*CheckoutService, error) {
	if opt.DefaultCurr == "" {
		opt.DefaultCurr = "RUB"
	}
	cli, err := NewOrdersGRPCClient(ctx, opt.OrdersAddr, opt.OrdersTO, opt.Logger)
	if err != nil {
		return nil, err
	}
	return &CheckoutService{
		log:         opt.Logger,
		orders:      cli,
		defaultCurr: opt.DefaultCurr,
	}, nil
}

func (s *CheckoutService) CreateOrder(ctx context.Context, in *checkoutpb.CreateOrderRequest) (*checkoutpb.CreateOrderResponse, error) {
	if in == nil || in.UserId == "" || in.AmountCents <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and positive amount_cents are required")
	}
	curr := in.Currency
	if curr == "" {
		curr = s.defaultCurr
	}

	ord, err := s.orders.CreateOrder(ctx, in.UserId, in.AmountCents, curr)
	if err != nil {
		s.log.Warn("orders.CreateOrder failed", "err", err)
		return nil, status.Errorf(codes.FailedPrecondition, "orders create failed: %v", err)
	}

	var st checkoutpb.OrderStatus
	switch ord.GetStatus() {
	case orderspb.OrderStatus_ORDER_STATUS_NEW:
		st = checkoutpb.OrderStatus_ORDER_STATUS_NEW
	case orderspb.OrderStatus_ORDER_STATUS_PAID:
		st = checkoutpb.OrderStatus_ORDER_STATUS_PAID
	case orderspb.OrderStatus_ORDER_STATUS_CANCELLED:
		st = checkoutpb.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		st = checkoutpb.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}

	return &checkoutpb.CreateOrderResponse{
		OrderId:     ord.GetOrderId(),
		Status:      st,
		Currency:    ord.GetCurrency(),
		TotalAmount: ord.GetTotalAmount(),
		CreatedAt:   ord.GetCreatedAt(),
	}, nil
}

func (s *CheckoutService) GetOrder(context.Context, *checkoutpb.GetOrderRequest) (*checkoutpb.GetOrderResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetOrder is not implemented yet")
}
