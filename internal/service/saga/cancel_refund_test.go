package saga

import (
	"errors"
	"testing"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

func TestOrchestrator_Cancel_FromReserved(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	// Seed order in Reserved status
	order := seedOrder(t, repo, domain.OrderStatusReserved)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Cancel(order.ID, "customer request")

	// Check order status
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusCanceled {
		t.Errorf("expected status Canceled, got %s", updated.Status)
	}

	// Check inventory was released
	if inv.releaseCnt != 1 {
		t.Errorf("expected 1 release call, got %d", inv.releaseCnt)
	}

	// Payment should not be refunded (not paid yet)
	if pay.refundCnt != 0 {
		t.Errorf("expected 0 refund calls, got %d", pay.refundCnt)
	}
}

func TestOrchestrator_Cancel_FromPaid(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{refundStatus: domain.PaymentStatusRefunded}

	// Seed order in Paid status
	order := seedOrder(t, repo, domain.OrderStatusPaid)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Cancel(order.ID, "customer request")

	// Check order status
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusCanceled {
		t.Errorf("expected status Canceled, got %s", updated.Status)
	}

	// Check inventory was released
	if inv.releaseCnt != 1 {
		t.Errorf("expected 1 release call, got %d", inv.releaseCnt)
	}

	// Payment should be refunded
	if pay.refundCnt != 1 {
		t.Errorf("expected 1 refund call, got %d", pay.refundCnt)
	}
}

func TestOrchestrator_Cancel_FromConfirmed(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{refundStatus: domain.PaymentStatusRefunded}

	// Seed order in Confirmed status
	order := seedOrder(t, repo, domain.OrderStatusConfirmed)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Cancel(order.ID, "customer request")

	// Check order status
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusCanceled {
		t.Errorf("expected status Canceled, got %s", updated.Status)
	}

	// Both inventory and payment should be compensated
	if inv.releaseCnt != 1 {
		t.Errorf("expected 1 release call, got %d", inv.releaseCnt)
	}

	if pay.refundCnt != 1 {
		t.Errorf("expected 1 refund call, got %d", pay.refundCnt)
	}
}

func TestOrchestrator_Cancel_AlreadyCanceled(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	// Seed order already canceled
	order := seedOrder(t, repo, domain.OrderStatusCanceled)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Cancel(order.ID, "customer request")

	// Should not do anything
	if inv.releaseCnt != 0 {
		t.Errorf("expected 0 release calls for already canceled order, got %d", inv.releaseCnt)
	}

	if pay.refundCnt != 0 {
		t.Errorf("expected 0 refund calls for already canceled order, got %d", pay.refundCnt)
	}
}

func TestOrchestrator_Cancel_OrderNotFound(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	
	// Try to cancel non-existent order
	orch.Cancel("non-existent", "test")

	// Should not panic, just log warning
	if inv.releaseCnt != 0 {
		t.Errorf("expected 0 release calls, got %d", inv.releaseCnt)
	}
}

func TestOrchestrator_Cancel_RefundFails(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{refundErr: errors.New("refund failed")}

	// Seed order in Paid status
	order := seedOrder(t, repo, domain.OrderStatusPaid)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Cancel(order.ID, "customer request")

	// Order should remain in Paid status if refund fails
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status == domain.OrderStatusCanceled {
		t.Error("order should not be canceled if refund fails")
	}
}

func TestOrchestrator_Refund_Success(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{refundStatus: domain.PaymentStatusRefunded}

	// Seed order in Confirmed status
	order := seedOrder(t, repo, domain.OrderStatusConfirmed)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Refund(order.ID, 100, "customer request")

	// Check order status
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusRefunded {
		t.Errorf("expected status Refunded, got %s", updated.Status)
	}

	// Check refund was called
	if pay.refundCnt != 1 {
		t.Errorf("expected 1 refund call, got %d", pay.refundCnt)
	}

	// Inventory should be released
	if inv.releaseCnt != 1 {
		t.Errorf("expected 1 release call, got %d", inv.releaseCnt)
	}
}

func TestOrchestrator_Refund_AlreadyRefunded(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	// Seed order already refunded
	order := seedOrder(t, repo, domain.OrderStatusRefunded)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Refund(order.ID, 100, "customer request")

	// Should not do anything
	if pay.refundCnt != 0 {
		t.Errorf("expected 0 refund calls for already refunded order, got %d", pay.refundCnt)
	}

	if inv.releaseCnt != 0 {
		t.Errorf("expected 0 release calls for already refunded order, got %d", inv.releaseCnt)
	}
}

func TestOrchestrator_Refund_WrongStatus(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	// Seed order in Pending status (not paid yet)
	order := seedOrder(t, repo, domain.OrderStatusPending)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	orch.Refund(order.ID, 100, "customer request")

	// Should not refund order that's not paid
	if pay.refundCnt != 0 {
		t.Errorf("expected 0 refund calls for pending order, got %d", pay.refundCnt)
	}

	// Order status should not change
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusPending {
		t.Errorf("expected status to remain Pending, got %s", updated.Status)
	}
}

func TestOrchestrator_Refund_OrderNotFound(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{}

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	
	// Try to refund non-existent order
	orch.Refund("non-existent", 100, "test")

	// Should not panic, just log warning
	if pay.refundCnt != 0 {
		t.Errorf("expected 0 refund calls, got %d", pay.refundCnt)
	}
}

func TestOrchestrator_Refund_PartialAmount(t *testing.T) {
	repo := memory.NewOrderRepository()
	outbox := memory.NewOutboxRepository()
	timeline := memory.NewTimelineRepository()
	inv := &stubInventory{}
	pay := &stubPayment{refundStatus: domain.PaymentStatusRefunded}

	// Seed order in Confirmed status with amount 100
	order := seedOrder(t, repo, domain.OrderStatusConfirmed)

	orch := NewOrchestratorWithoutMetrics(repo, outbox, timeline, inv, pay, nil)
	
	// Refund partial amount (50 out of 100)
	orch.Refund(order.ID, 50, "partial refund")

	// Check refund was called
	if pay.refundCnt != 1 {
		t.Errorf("expected 1 refund call, got %d", pay.refundCnt)
	}

	// Order should be refunded
	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("failed to get order: %v", err)
	}

	if updated.Status != domain.OrderStatusRefunded {
		t.Errorf("expected status Refunded, got %s", updated.Status)
	}
}
