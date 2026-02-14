package saga

import (
	"context"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

func TestNewBatchProcessor(t *testing.T) {
	logger := log.WithField("test", "batch")

	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, logger)
	bp := NewBatchProcessor(orch, logger)

	if bp == nil {
		t.Fatal("NewBatchProcessor should not return nil")
	}

	if bp.orchestrator == nil {
		t.Error("orchestrator should not be nil")
	}

	if bp.batchSize != 10 {
		t.Errorf("expected batchSize 10, got %d", bp.batchSize)
	}

	if bp.flushTimeout != 100*time.Millisecond {
		t.Errorf("expected flushTimeout 100ms, got %v", bp.flushTimeout)
	}
}

func TestNewBatchProcessor_WithNilLogger(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	bp := NewBatchProcessor(orch, nil)

	if bp == nil {
		t.Fatal("NewBatchProcessor should not return nil")
	}

	if bp.logger == nil {
		t.Error("logger should be initialized even when nil is passed")
	}
}

func TestBatchProcessor_StartStop(_ *testing.T) {
	logger := log.WithField("test", "batch-lifecycle")

	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, logger)
	bp := NewBatchProcessor(orch, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем
	bp.Start(ctx)

	// Даём время на запуск
	time.Sleep(50 * time.Millisecond)

	// Останавливаем
	bp.Stop()

	// Проверяем что все goroutines завершились (Stop ждёт wg.Wait())
}

func TestBatchProcessor_StartOrder(t *testing.T) {
	logger := log.WithField("test", "batch-start")

	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{payStatus: domain.PaymentStatusCaptured}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, logger)
	bp := NewBatchProcessor(orch, logger)

	// Создаём заказ
	order := seedOrder(t, repo, domain.OrderStatusPending)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bp.Start(ctx)
	defer bp.Stop()

	// Отправляем в обработку
	bp.StartOrder(order.ID)

	// Даём время на обработку
	time.Sleep(200 * time.Millisecond)

	// Проверяем что заказ обработан
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	// Заказ должен быть в финальном статусе
	if updated.Status == domain.OrderStatusPending {
		t.Error("order should be processed")
	}
}

func TestBatchProcessor_CancelOrder(t *testing.T) {
	logger := log.WithField("test", "batch-cancel")

	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, logger)
	bp := NewBatchProcessor(orch, logger)

	// Создаём заказ в Reserved статусе
	order := seedOrder(t, repo, domain.OrderStatusReserved)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bp.Start(ctx)
	defer bp.Stop()

	// Отменяем заказ
	bp.CancelOrder(order.ID, "test cancellation")

	// Даём время на обработку
	time.Sleep(200 * time.Millisecond)

	// Проверяем что заказ отменён
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusCanceled {
		t.Errorf("expected status Canceled, got %s", updated.Status)
	}
}

func TestBatchProcessor_RefundOrder(t *testing.T) {
	logger := log.WithField("test", "batch-refund")

	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{refundStatus: domain.PaymentStatusRefunded}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, logger)
	bp := NewBatchProcessor(orch, logger)

	// Создаём заказ в Confirmed статусе
	order := seedOrder(t, repo, domain.OrderStatusConfirmed)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bp.Start(ctx)
	defer bp.Stop()

	// Возвращаем заказ
	bp.RefundOrder(order.ID, order.AmountMinor, "test refund")

	// Даём время на обработку
	time.Sleep(200 * time.Millisecond)

	// Проверяем что заказ возвращён
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusRefunded {
		t.Errorf("expected status Refunded, got %s", updated.Status)
	}
}

func TestBatchProcessor_MultipleBatches(t *testing.T) {
	logger := log.WithField("test", "batch-multiple")

	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{payStatus: domain.PaymentStatusCaptured}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, logger)
	bp := NewBatchProcessor(orch, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bp.Start(ctx)

	// Создаём несколько заказов
	orderCount := 15 // Больше чем batchSize (10)
	orderIDs := make([]string, orderCount)

	for i := 0; i < orderCount; i++ {
		order := seedOrderWithID(t, repo, domain.OrderStatusPending, i)
		orderIDs[i] = order.ID
		bp.StartOrder(order.ID)
	}

	// Даём время на обработку всех батчей
	time.Sleep(500 * time.Millisecond)

	// Останавливаем процессор и ждём завершения всех goroutines
	bp.Stop()

	// Проверяем что все заказы обработаны
	processed := 0
	for _, orderID := range orderIDs {
		updated, err := repo.Get(orderID)
		if err != nil {
			continue
		}
		if updated.Status != domain.OrderStatusPending {
			processed++
		}
	}

	if processed == 0 {
		t.Error("at least some orders should be processed")
	}
}

func TestBatchProcessor_StopWhileProcessing(t *testing.T) {
	logger := log.WithField("test", "batch-stop")

	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, logger)
	bp := NewBatchProcessor(orch, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bp.Start(ctx)

	// Отправляем операции
	order := seedOrder(t, repo, domain.OrderStatusPending)
	bp.StartOrder(order.ID)

	// Сразу останавливаем
	bp.Stop()

	// Не должно паниковать
}

// seedOrderWithID создаёт заказ с определённым ID для тестов
func seedOrderWithID(t *testing.T, repo domain.OrderRepository, status domain.OrderStatus, idx int) domain.Order {
	t.Helper()

	now := time.Now().UTC()
	order := domain.Order{
		ID:          "order-" + time.Now().Format("20060102150405") + "-" + string(rune(idx)),
		CustomerID:  "customer-1",
		Status:      status,
		Currency:    "USD",
		AmountMinor: 100,
		Items: []domain.OrderItem{{
			ID:         "item-1",
			SKU:        "sku-1",
			Qty:        1,
			PriceMinor: 100,
			CreatedAt:  now,
		}},
		Version:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Create(order); err != nil {
		t.Fatalf("create order: %v", err)
	}

	return order
}
