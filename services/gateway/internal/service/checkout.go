// services/gateway/internal/service/checkout.go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"strings"
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
	Redis       *redis.Client
}

type CheckoutService struct {
	checkoutpb.UnimplementedCheckoutServer
	log         *slog.Logger
	orders      *OrdersGRPCClient
	defaultCurr string
	rdb         *redis.Client
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
		rdb:         opt.Redis,
	}, nil
}

func (s *CheckoutService) CreateOrder(ctx context.Context, in *checkoutpb.CreateOrderRequest) (*checkoutpb.CreateOrderResponse, error) {
	// 0) валидация
	if in == nil || in.UserId == "" || in.AmountCents <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and positive amount_cents are required")
	}
	curr := in.Currency
	if curr == "" {
		curr = s.defaultCurr
	}

	// 1) вытащим идемпотентный ключ из gRPC metadata (оба варианта заголовка)
	var idemKeyStr string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for _, k := range []string{"idempotency-key", "x-idempotency-key"} {
			if v := md.Get(k); len(v) > 0 && strings.TrimSpace(v[0]) != "" {
				idemKeyStr = strings.TrimSpace(v[0])
				break
			}
		}
	}

	// если ключ не передан — обычный путь без идемпотентности
	if idemKeyStr == "" {
		out, err := s.orders.CreateOrder(ctx, in.UserId, in.AmountCents, curr)
		if err != nil {
			s.log.Warn("orders.CreateOrder failed", "err", err)
			return nil, status.Errorf(codes.FailedPrecondition, "orders create failed: %v", err)
		}
		return &checkoutpb.CreateOrderResponse{
			OrderId:     out.GetOrderId(),
			Status:      mapOrdersStatus(out.GetStatus()),
			Currency:    out.GetCurrency(),
			TotalAmount: out.GetTotalAmount(),
			CreatedAt:   out.GetCreatedAt(),
		}, nil
	}

	// 2) ключи в Redis
	idemKey := fmt.Sprintf("idem:checkout:create:%s", idemKeyStr) // HASH с состоянием и ответом
	lockKey := idemKey + ":lock"
	lockTTL := 15 * time.Second
	runTTL := 60 * time.Second // пока выполняется первый запрос
	finalTTL := time.Hour      // TTL успешного результата
	nowRFC3339 := time.Now().UTC().Format(time.RFC3339)
	ph := payloadHash(in.UserId, in.AmountCents, curr)

	// 3) быстрый путь — ключ уже есть
	m, err := s.rdb.HGetAll(ctx, idemKey).Result()
	if err == nil && len(m) > 0 {
		// конфликт: тот же idem-key, другой payload
		if old := m["payload_hash"]; old != "" && old != ph {
			return nil, status.Error(codes.AlreadyExists, "idempotency key reused with different payload")
		}
		switch m["state"] {
		case "done":
			if js := m["resp"]; js != "" {
				var out checkoutpb.CreateOrderResponse
				if err := protojson.Unmarshal([]byte(js), &out); err == nil {
					s.log.Info("idem: replay", "key", idemKeyStr)
					return &out, nil
				}
			}
			// если не смогли распарсить — пойдём обычным путём ниже
		case "in_progress":
			return nil, status.Error(codes.Aborted, "idempotent request is in progress, retry later")
		}
	} else if err != nil {
		s.log.Warn("redis HGetAll failed", "key", idemKey, "err", err)
	}

	// 4) пытаемся захватить лок (SETNX)
	ok, err := s.rdb.SetNX(ctx, lockKey, "1", lockTTL).Result()
	if err != nil {
		s.log.Warn("redis SetNX lock failed", "key", lockKey, "err", err)
		return nil, status.Error(codes.ResourceExhausted, "idempotency lock failed")
	}
	if !ok {
		return nil, status.Error(codes.Aborted, "idempotent request is in progress, retry later")
	}
	defer func() {
		// на выходе всегда снимаем лок (best effort)
		_ = s.rdb.Del(context.Background(), lockKey).Err()
	}()

	// 5) помечаем запрос как in_progress + фиксируем payload_hash
	if err := s.rdb.HSet(ctx, idemKey, map[string]any{
		"state":        "in_progress",
		"payload_hash": ph,
		"ts":           nowRFC3339,
	}).Err(); err != nil {
		s.log.Warn("redis HSet(in_progress) failed", "key", idemKey, "err", err)
	}
	_ = s.rdb.Expire(ctx, idemKey, runTTL).Err()

	s.log.Info("idem: commit", "key", idemKeyStr)

	// 6) основной вызов в orders
	out, err := s.orders.CreateOrder(ctx, in.UserId, in.AmountCents, curr)
	if err != nil {
		s.log.Warn("orders.CreateOrder failed", "err", err)
		// можно сохранить state=error с коротким TTL (опционально)
		_ = s.rdb.HSet(ctx, idemKey, map[string]any{
			"state":        "error",
			"payload_hash": ph,
			"code":         "ERROR",
			"err":          err.Error(),
			"ts":           time.Now().UTC().Format(time.RFC3339),
		}).Err()
		_ = s.rdb.Expire(ctx, idemKey, 30*time.Second).Err()
		return nil, status.Errorf(codes.FailedPrecondition, "orders create failed: %v", err)
	}

	resp := &checkoutpb.CreateOrderResponse{
		OrderId:     out.GetOrderId(),
		Status:      mapOrdersStatus(out.GetStatus()),
		Currency:    out.GetCurrency(),
		TotalAmount: out.GetTotalAmount(),
		CreatedAt:   out.GetCreatedAt(),
	}

	js, _ := protojson.Marshal(resp)
	if err := s.rdb.HSet(ctx, idemKey, map[string]any{
		"state":        "done",
		"payload_hash": ph,
		"resp":         string(js),
		"code":         "OK",
		"ts":           time.Now().UTC().Format(time.RFC3339),
	}).Err(); err != nil {
		s.log.Warn("redis HSet(done) failed", "key", idemKey, "err", err)
	}
	_ = s.rdb.Expire(ctx, idemKey, finalTTL).Err()

	return resp, nil
}

// --- helpers ---

func payloadHash(userID string, amountCents int64, currency string) string {
	c := strings.ToUpper(strings.TrimSpace(currency))
	base := fmt.Sprintf("%s|%d|%s", strings.TrimSpace(userID), amountCents, c)
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:])
}

func mapOrdersStatus(s orderspb.OrderStatus) checkoutpb.OrderStatus {
	switch s {
	case orderspb.OrderStatus_ORDER_STATUS_NEW:
		return checkoutpb.OrderStatus_ORDER_STATUS_NEW
	case orderspb.OrderStatus_ORDER_STATUS_PAID:
		return checkoutpb.OrderStatus_ORDER_STATUS_PAID
	case orderspb.OrderStatus_ORDER_STATUS_CANCELLED:
		return checkoutpb.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return checkoutpb.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func (s *CheckoutService) GetOrder(context.Context, *checkoutpb.GetOrderRequest) (*checkoutpb.GetOrderResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetOrder is not implemented yet")
}
