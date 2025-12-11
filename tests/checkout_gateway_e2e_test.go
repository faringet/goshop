//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"goshop/services/gateway/api/checkoutpb"
)

// gatewayAddr возвращает адрес gRPC-gateway.
// По умолчанию — localhost:7071 (как в docker-compose.gateway.yml ports: "7071:7070").
// Можно переопределить через переменную окружения GOSHOP_GATEWAY_GRPC_ADDR.
func gatewayAddr(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("GOSHOP_GATEWAY_GRPC_ADDR"); v != "" {
		return v
	}
	return "localhost:7071"
}

// TestCheckout_CreateOrderAndPay_viaGateway:
//  1. вызывает Checkout.CreateOrder через gateway;
//  2. проверяет, что order_id не пустой и статус адекватный (NEW или PAID);
//  3. опрашивает Checkout.GetOrderStatus, пока статус не станет PAID.
//
// Для прохождения теста должны быть подняты:
// - postgres, redis, kafka
// - orders (+ orders-migrate, orders-consumer)
// - payments (+ payments-migrate, payments-consumer)
// - outboxer
// - gateway
func TestCheckout_CreateOrderAndPay_viaGateway(t *testing.T) {
	addr := gatewayAddr(t)

	// Общий дедлайн на весь сценарий
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// gRPC-коннект к gateway
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial gateway at %q failed: %v", addr, err)
	}
	defer conn.Close()

	client := checkoutpb.NewCheckoutClient(conn)

	// 1) CreateOrder
	userID := uuid.NewString()
	amountCents := int64(19901) // у меня логика что если сумма кратна 5 - failed

	createCtx, cancelCreate := context.WithTimeout(ctx, 5*time.Second)
	defer cancelCreate()

	createResp, err := client.CreateOrder(createCtx, &checkoutpb.CreateOrderRequest{
		UserId:      userID,
		AmountCents: amountCents,
		Currency:    "RUB",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	if createResp.OrderId == "" {
		t.Fatalf("CreateOrder: empty order_id in response: %#v", createResp)
	}

	// Начальный статус: обычно NEW, но допускаем, что система может уже успеть проставить PAID.
	if createResp.Status != checkoutpb.OrderStatus_ORDER_STATUS_NEW &&
		createResp.Status != checkoutpb.OrderStatus_ORDER_STATUS_PAID {
		t.Fatalf("CreateOrder: unexpected initial status=%s", createResp.Status.String())
	}

	t.Logf("created order %s, initial status=%s", createResp.OrderId, createResp.Status.String())

	orderID := createResp.OrderId

	// 2) Опрашиваем GetOrderStatus, пока не станет PAID
	var finalStatus checkoutpb.OrderStatus
	deadline := time.Now().Add(20 * time.Second)

	for time.Now().Before(deadline) {
		stCtx, cancelStatus := context.WithTimeout(ctx, 3*time.Second)
		statusResp, err := client.GetOrderStatus(stCtx, &checkoutpb.GetOrderStatusRequest{
			OrderId: orderID,
		})
		cancelStatus()
		if err != nil {
			t.Fatalf("GetOrderStatus failed: %v", err)
		}

		finalStatus = statusResp.Status
		t.Logf("GetOrderStatus: order=%s status=%s", orderID, finalStatus.String())

		if finalStatus == checkoutpb.OrderStatus_ORDER_STATUS_PAID {
			break
		}

		// Немного подождать, чтобы дать время цепочке Kafka/payments/orders обновить статус
		time.Sleep(500 * time.Millisecond)
	}

	if finalStatus != checkoutpb.OrderStatus_ORDER_STATUS_PAID {
		t.Fatalf("order %s final status=%s, want PAID. "+
			"Проверь, что запущены payments, orders-consumer, payments-consumer, outboxer и Kafka.",
			orderID, finalStatus.String())
	}
}
