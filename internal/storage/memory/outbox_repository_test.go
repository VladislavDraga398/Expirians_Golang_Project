package memory

import (
	"testing"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestOutboxRepository_EnqueueAndPull(t *testing.T) {
	repo := NewOutboxRepository()

	msg := domain.OutboxMessage{
		AggregateType: "order",
		AggregateID:   "order-1",
		EventType:     "OrderStatusChanged",
		Payload:       []byte(`{"status":"pending"}`),
	}

	saved, err := repo.Enqueue(msg)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("expected generated id")
	}

	pending, err := repo.PullPending(10)
	if err != nil {
		t.Fatalf("pull pending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending message, got %d", len(pending))
	}
	if pending[0].ID != saved.ID {
		t.Fatalf("expected same message id, got %s", pending[0].ID)
	}
}

func TestOutboxRepository_MarkSentAndFailed(t *testing.T) {
	repo := NewOutboxRepository()

	saved, err := repo.Enqueue(domain.OutboxMessage{AggregateType: "order"})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	if err := repo.MarkSent(saved.ID); err != nil {
		t.Fatalf("mark sent failed: %v", err)
	}

	if err := repo.MarkFailed(saved.ID); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	if err := repo.MarkFailed("missing"); err == nil {
		t.Fatal("expected error for missing record")
	}
}

func TestOutboxRepository_Stats(t *testing.T) {
	repo := NewOutboxRepository()

	first, err := repo.Enqueue(domain.OutboxMessage{AggregateType: "order", AggregateID: "order-1"})
	if err != nil {
		t.Fatalf("enqueue first failed: %v", err)
	}
	_, err = repo.Enqueue(domain.OutboxMessage{AggregateType: "order", AggregateID: "order-2"})
	if err != nil {
		t.Fatalf("enqueue second failed: %v", err)
	}
	if err := repo.MarkSent(first.ID); err != nil {
		t.Fatalf("mark sent failed: %v", err)
	}

	stats, err := repo.Stats()
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.PendingCount != 1 {
		t.Fatalf("expected 1 pending message, got %d", stats.PendingCount)
	}
	if stats.OldestPendingAt.IsZero() {
		t.Fatal("expected oldest pending timestamp")
	}
}
