package integration

import (
	"context"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	grpcsvc "github.com/vladislavdragonenkov/oms/internal/service/grpc"
	"github.com/vladislavdragonenkov/oms/internal/service/inventory"
	"github.com/vladislavdragonenkov/oms/internal/service/payment"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

// OrderLifecycleTestSuite тестирует полный жизненный цикл заказов.
type OrderLifecycleTestSuite struct {
	suite.Suite
	service   *grpcsvc.OrderService
	repo      domain.OrderRepository
	timeline  domain.TimelineRepository
	inventory *inventory.MockService
	payment   *payment.MockService
	saga      saga.Orchestrator
}

func (suite *OrderLifecycleTestSuite) SetupTest() {
	baseLogger := log.New()
	baseLogger.SetLevel(log.WarnLevel) // Уменьшаем шум в тестах
	logger := baseLogger.WithField("component", "integration-test")

	suite.repo = memory.NewOrderRepository()
	suite.timeline = memory.NewTimelineRepository()
	outbox := memory.NewOutboxRepository()

	suite.inventory = inventory.NewMockService()
	suite.payment = payment.NewMockService()

	suite.saga = saga.NewOrchestratorWithoutMetrics(
		suite.repo,
		outbox,
		suite.timeline,
		suite.inventory,
		suite.payment,
		logger,
	)

	suite.service = grpcsvc.NewOrderService(
		suite.repo,
		suite.timeline,
		suite.saga,
		logger,
	)
}

func (suite *OrderLifecycleTestSuite) TestSuccessfulOrderLifecycle() {
	ctx := context.Background()

	// 1. Создаём заказ
	createResp, err := suite.service.CreateOrder(ctx, &omsv1.CreateOrderRequest{
		CustomerId: "customer-123",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku: "laptop-pro",
				Qty: 1,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 199900, // $1999.00
				},
			},
			{
				Sku: "mouse-wireless",
				Qty: 2,
				Price: &omsv1.Money{
					Currency:    "USD",
					AmountMinor: 4999, // $49.99
				},
			},
		},
	})

	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), createResp.Order)
	require.Equal(suite.T(), omsv1.OrderStatus_ORDER_STATUS_PENDING, createResp.Order.Status)
	require.Equal(suite.T(), int64(209898), createResp.Order.Amount.AmountMinor) // $1999 + 2*$49.99

	orderID := createResp.Order.Id

	// 2. Инициируем платёж
	payResp, err := suite.service.PayOrder(ctx, &omsv1.PayOrderRequest{
		OrderId: orderID,
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), orderID, payResp.OrderId)

	// Ждём завершения саги
	suite.waitForOrderStatus(orderID, domain.OrderStatusConfirmed, 5*time.Second)

	// 3. Проверяем финальное состояние
	getResp, err := suite.service.GetOrder(ctx, &omsv1.GetOrderRequest{
		OrderId: orderID,
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), omsv1.OrderStatus_ORDER_STATUS_CONFIRMED, getResp.Order.Status)

	// 4. Проверяем timeline
	require.NotNil(suite.T(), getResp.Timeline)
	require.GreaterOrEqual(suite.T(), len(getResp.Timeline), 4) // Минимум 4 события: pending->reserved->paid->confirmed

	// 5. Проверяем вызовы внешних сервисов
	require.Equal(suite.T(), 1, suite.inventory.ReserveCalls)
	require.Equal(suite.T(), 1, suite.payment.PayCalls)
	require.Equal(suite.T(), 0, suite.inventory.ReleaseCalls) // Не должно быть отмен
	require.Equal(suite.T(), 0, suite.payment.RefundCalls)    // Не должно быть возвратов
}

func (suite *OrderLifecycleTestSuite) TestOrderCancellation() {
	ctx := context.Background()

	// 1. Создаём и оплачиваем заказ
	orderID := suite.createAndPayOrder(ctx)

	// 2. Отменяем заказ
	_, err := suite.service.CancelOrder(ctx, &omsv1.CancelOrderRequest{
		OrderId: orderID,
		Reason:  "Customer changed mind",
	})
	require.NoError(suite.T(), err)

	// 3. Ждём завершения компенсаций и проверяем статус
	updatedOrder := suite.waitForOrderStatusViaGRPC(ctx, orderID, omsv1.OrderStatus_ORDER_STATUS_CANCELED, 2*time.Second)

	// 4. Проверяем компенсации
	require.Equal(suite.T(), 1, suite.inventory.ReleaseCalls) // Освобождён резерв
	require.Equal(suite.T(), 1, suite.payment.RefundCalls)    // Возвращены деньги

	// 4. Проверяем timeline события
	hasCancel := false
	for _, event := range updatedOrder.Timeline {
		if event.Type == "OrderCanceled" {
			hasCancel = true
			require.Equal(suite.T(), "Customer changed mind", event.Reason)
		}
	}
	require.True(suite.T(), hasCancel, "Timeline should contain OrderCanceled event")
}

func (suite *OrderLifecycleTestSuite) TestPartialRefund() {
	ctx := context.Background()

	// 1. Создаём и подтверждаем заказ
	orderID := suite.createConfirmedOrder(ctx)

	// 2. Частичный возврат
	refundAmount := int64(50000) // $500.00 из $2098.98
	_, err := suite.service.RefundOrder(ctx, &omsv1.RefundOrderRequest{
		OrderId: orderID,
		Amount: &omsv1.Money{
			Currency:    "USD",
			AmountMinor: refundAmount,
		},
		Reason: "Partial return - one item damaged",
	})
	require.NoError(suite.T(), err)

	// 3. Ждём завершения и проверяем статус
	updatedOrder := suite.waitForOrderStatusViaGRPC(ctx, orderID, omsv1.OrderStatus_ORDER_STATUS_REFUNDED, 2*time.Second)

	// 4. Проверяем вызовы
	require.Equal(suite.T(), 1, suite.payment.RefundCalls)
	require.Equal(suite.T(), 1, suite.inventory.ReleaseCalls) // Весь резерв освобождается

	// 5. Проверяем timeline события
	hasRefund := false
	for _, event := range updatedOrder.Timeline {
		if event.Type == "OrderRefunded" {
			hasRefund = true
			require.Equal(suite.T(), "Partial return - one item damaged", event.Reason)
		}
	}
	require.True(suite.T(), hasRefund, "Timeline should contain OrderRefunded event")
}

func (suite *OrderLifecycleTestSuite) TestInventoryFailureCompensation() {
	ctx := context.Background()

	// Настраиваем сбой резерва
	suite.inventory.ReserveErr = domain.ErrInventoryUnavailable

	// 1. Создаём заказ
	createResp, err := suite.service.CreateOrder(ctx, &omsv1.CreateOrderRequest{
		CustomerId: "customer-456",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{
				Sku:   "out-of-stock-item",
				Qty:   1,
				Price: &omsv1.Money{Currency: "USD", AmountMinor: 10000},
			},
		},
	})
	require.NoError(suite.T(), err)
	orderID := createResp.Order.Id

	// 2. Инициируем платёж
	_, err = suite.service.PayOrder(ctx, &omsv1.PayOrderRequest{OrderId: orderID})
	require.NoError(suite.T(), err)

	// Ждём завершения саги с ошибкой
	suite.waitForOrderStatus(orderID, domain.OrderStatusCanceled, 5*time.Second)

	// 3. Проверяем компенсацию
	order, err := suite.repo.Get(orderID)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), domain.OrderStatusCanceled, order.Status)

	// 4. Проверяем, что платёж не был инициирован
	require.Equal(suite.T(), 1, suite.inventory.ReserveCalls)
	require.Equal(suite.T(), 0, suite.payment.PayCalls) // Платёж не должен был произойти
}

func (suite *OrderLifecycleTestSuite) TestPaymentFailureCompensation() {
	ctx := context.Background()

	// Настраиваем сбой платежа
	suite.payment.PayErr = domain.ErrPaymentDeclined

	// 1. Создаём заказ и инициируем платёж
	orderID := suite.createOrderAndPay(ctx)

	// Ждём завершения саги с ошибкой
	suite.waitForOrderStatus(orderID, domain.OrderStatusCanceled, 5*time.Second)

	// 2. Проверяем компенсацию
	order, err := suite.repo.Get(orderID)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), domain.OrderStatusCanceled, order.Status)

	// 3. Проверяем, что резерв был освобождён
	require.Equal(suite.T(), 1, suite.inventory.ReserveCalls)
	require.Equal(suite.T(), 1, suite.inventory.ReleaseCalls) // Компенсация
	require.Equal(suite.T(), 1, suite.payment.PayCalls)
}

// Вспомогательные методы

func (suite *OrderLifecycleTestSuite) createAndPayOrder(ctx context.Context) string {
	createResp, err := suite.service.CreateOrder(ctx, &omsv1.CreateOrderRequest{
		CustomerId: "customer-789",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{Sku: "test-item", Qty: 1, Price: &omsv1.Money{Currency: "USD", AmountMinor: 10000}},
		},
	})
	require.NoError(suite.T(), err)

	orderID := createResp.Order.Id
	_, err = suite.service.PayOrder(ctx, &omsv1.PayOrderRequest{OrderId: orderID})
	require.NoError(suite.T(), err)

	suite.waitForOrderStatus(orderID, domain.OrderStatusConfirmed, 5*time.Second)
	return orderID
}

func (suite *OrderLifecycleTestSuite) createConfirmedOrder(ctx context.Context) string {
	return suite.createAndPayOrder(ctx)
}

func (suite *OrderLifecycleTestSuite) createOrderAndPay(ctx context.Context) string {
	createResp, err := suite.service.CreateOrder(ctx, &omsv1.CreateOrderRequest{
		CustomerId: "customer-fail",
		Currency:   "USD",
		Items: []*omsv1.OrderItem{
			{Sku: "fail-item", Qty: 1, Price: &omsv1.Money{Currency: "USD", AmountMinor: 5000}},
		},
	})
	require.NoError(suite.T(), err)

	orderID := createResp.Order.Id
	_, err = suite.service.PayOrder(ctx, &omsv1.PayOrderRequest{OrderId: orderID})
	require.NoError(suite.T(), err)

	return orderID
}

func (suite *OrderLifecycleTestSuite) waitForOrderStatus(orderID string, expectedStatus domain.OrderStatus, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		order, err := suite.repo.Get(orderID)
		if err == nil && order.Status == expectedStatus {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Если не дождались, показываем текущий статус
	order, _ := suite.repo.Get(orderID)
	suite.T().Fatalf("Order %s did not reach status %s within %v, current status: %s",
		orderID, expectedStatus, timeout, order.Status)
}

// waitForOrderStatusViaGRPC ждёт статус через gRPC API
func (suite *OrderLifecycleTestSuite) waitForOrderStatusViaGRPC(ctx context.Context, orderID string, expectedStatus omsv1.OrderStatus, timeout time.Duration) *omsv1.GetOrderResponse {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := suite.service.GetOrder(ctx, &omsv1.GetOrderRequest{OrderId: orderID})
		if err == nil && resp.Order.Status == expectedStatus {
			return resp
		}
		time.Sleep(50 * time.Millisecond) // Чуть больше интервал для gRPC
	}

	// Последняя попытка для диагностики
	resp, _ := suite.service.GetOrder(ctx, &omsv1.GetOrderRequest{OrderId: orderID})
	if resp != nil {
		suite.T().Fatalf("Order %s did not reach status %s within %v, current status: %s",
			orderID, expectedStatus, timeout, resp.Order.Status)
	} else {
		suite.T().Fatalf("Order %s not found or error occurred", orderID)
	}
	return nil
}

func TestOrderLifecycle(t *testing.T) {
	suite.Run(t, new(OrderLifecycleTestSuite))
}
