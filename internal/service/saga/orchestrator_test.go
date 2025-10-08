package saga

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

type stubInventory struct {
	mu         sync.Mutex
	reserveErr error
	releaseErr error
	reserveCnt int
	releaseCnt int
}

func (s *stubInventory) Reserve(orderID string, items []domain.OrderItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reserveCnt++
	return s.reserveErr
}

func (s *stubInventory) Release(orderID string, items []domain.OrderItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseCnt++
	return s.releaseErr
}

type stubPayment struct {
	mu           sync.Mutex
	payStatus    domain.PaymentStatus
	payErr       error
	refundStatus domain.PaymentStatus
	refundErr    error

	payCnt    int
	refundCnt int
}

func (s *stubPayment) Pay(orderID string, amountMinor int64, currency string) (domain.PaymentStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payCnt++
	return s.payStatus, s.payErr
}

func (s *stubPayment) Refund(orderID string, amountMinor int64, currency string) (domain.PaymentStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refundCnt++
	return s.refundStatus, s.refundErr
}

func seedOrder(t *testing.T, repo domain.OrderRepository, status domain.OrderStatus) domain.Order {
	t.Helper()

	now := time.Now().UTC()
	order := domain.Order{
		ID:          "order-1",
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

func collectOutbox(t *testing.T, outbox domain.OutboxRepository) []domain.OutboxMessage {
	t.Helper()

	type allPending interface {
		AllPending() []domain.OutboxMessage
	}

	repo, ok := outbox.(allPending)
	if !ok {
		t.Fatalf("outbox repository does not support AllPending")
	}

	return repo.AllPending()
}

func decodeStatus(t *testing.T, msg domain.OutboxMessage) string {
	t.Helper()

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	status, _ := payload["status"].(string)
	return status
}

func TestOrchestrator_SuccessFlow(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inventory := &stubInventory{}
	payments := &stubPayment{payStatus: domain.PaymentStatusCaptured}

	seedOrder(t, repo, domain.OrderStatusPending)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inventory, payments, log.New().WithField("test", "success"))
	orch.Start("order-1")

	updated, err := repo.Get("order-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}

	if updated.Status != domain.OrderStatusConfirmed {
		t.Fatalf("expected status confirmed, got %s", updated.Status)
	}

	if inventory.reserveCnt != 1 {
		t.Fatalf("expected reserve called once, got %d", inventory.reserveCnt)
	}

	if payments.payCnt != 1 {
		t.Fatalf("expected pay called once, got %d", payments.payCnt)
	}

	events := collectOutbox(t, outbox)
	if len(events) < 3 {
		t.Fatalf("expected at least 3 outbox events, got %d", len(events))
	}

	// Проверяем, что заказ дошёл до финального статуса
	if updated.Status != domain.OrderStatusConfirmed {
		t.Fatalf("expected final status %s, got %s", domain.OrderStatusConfirmed, updated.Status)
	}

	// Проверяем, что есть события статуса (количество может варьироваться)
	if len(events) == 0 {
		t.Fatal("expected at least one status event")
	}
}

func TestOrchestrator_ReserveFailure(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inventory := &stubInventory{reserveErr: domain.ErrInventoryUnavailable}
	payments := &stubPayment{payStatus: domain.PaymentStatusCaptured}

	seedOrder(t, repo, domain.OrderStatusPending)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inventory, payments, log.New().WithField("test", "reserve_failure"))
	orch.Start("order-1")

	updated, err := repo.Get("order-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}

	if updated.Status != domain.OrderStatusCanceled {
		t.Fatalf("expected status canceled, got %s", updated.Status)
	}

	if inventory.reserveCnt != 1 {
		t.Fatalf("expected reserve called once, got %d", inventory.reserveCnt)
	}

	if payments.payCnt != 0 {
		t.Fatalf("expected pay not called, got %d", payments.payCnt)
	}

	events := collectOutbox(t, outbox)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 outbox events, got %d", len(events))
	}

	// Проверяем, что есть событие failed
	hasFailedEvent := false
	for _, event := range events {
		if event.EventType == "OrderSagaFailed" {
			hasFailedEvent = true
			break
		}
	}
	if !hasFailedEvent {
		t.Fatal("expected OrderSagaFailed event")
	}
}

func TestOrchestrator_PaymentFailure(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inventory := &stubInventory{}
	payments := &stubPayment{payErr: domain.ErrPaymentDeclined}

	seedOrder(t, repo, domain.OrderStatusPending)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inventory, payments, log.New().WithField("test", "payment_failure"))
	orch.Start("order-1")

	updated, err := repo.Get("order-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}

	if updated.Status != domain.OrderStatusCanceled {
		t.Fatalf("expected status canceled, got %s", updated.Status)
	}

	if inventory.releaseCnt != 1 {
		t.Fatalf("expected release called once, got %d", inventory.releaseCnt)
	}

	events := collectOutbox(t, outbox)
	if len(events) < 3 {
		t.Fatalf("expected at least 3 outbox events, got %d", len(events))
	}

	// Проверяем, что есть событие failed
	hasFailedEvent := false
	for _, event := range events {
		if event.EventType == "OrderSagaFailed" {
			hasFailedEvent = true
			break
		}
	}
	if !hasFailedEvent {
		t.Fatal("expected OrderSagaFailed event")
	}
}
