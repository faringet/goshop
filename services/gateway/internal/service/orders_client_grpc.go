package service

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	orderpb "goshop/services/orders/api/orderspb"
)

type OrdersGRPCClient struct {
	cc      *grpc.ClientConn
	cli     orderpb.OrdersClient
	log     *slog.Logger
	timeout time.Duration
}

func NewOrdersGRPCClient(ctx context.Context, addr string, timeout time.Duration, log *slog.Logger) (*OrdersGRPCClient, error) {
	if log == nil {
		log = slog.Default()
	}

	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	cc, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithDisableRetry(), // чтобы быстрее фейлиться при недоступности
	)
	if err != nil {
		return nil, fmt.Errorf("dial orders-grpc %s: %w", addr, err)
	}

	return &OrdersGRPCClient{
		cc:      cc,
		cli:     orderpb.NewOrdersClient(cc),
		log:     log,
		timeout: timeout,
	}, nil
}

func (c *OrdersGRPCClient) Close() error { return c.cc.Close() }

func (c *OrdersGRPCClient) CreateOrder(ctx context.Context, userID string, amountCents int64, currency string) (*orderpb.CreateOrderResponse, error) {
	rctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &orderpb.CreateOrderRequest{
		UserId:      userID,
		AmountCents: amountCents,
		Currency:    currency,
	}
	return c.cli.CreateOrder(rctx, req)
}
