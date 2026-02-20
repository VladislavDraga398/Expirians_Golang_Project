package memory

import (
	"testing"
	"time"

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

func TestOutboxRepository_PullPendingClaimsMessage(t *testing.T) {
	repo := NewOutboxRepository()

	saved, err := repo.Enqueue(domain.OutboxMessage{
		AggregateType: "order",
		AggregateID:   "order-claim",
		EventType:     "OrderCreated",
	})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	first, err := repo.PullPending(10)
	if err != nil {
		t.Fatalf("first pull failed: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 claimed message, got %d", len(first))
	}
	if first[0].ID != saved.ID {
		t.Fatalf("expected claimed id %s, got %s", saved.ID, first[0].ID)
	}

	second, err := repo.PullPending(10)
	if err != nil {
		t.Fatalf("second pull failed: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("expected message to be locked in processing, got %d messages", len(second))
	}

	stats, err := repo.Stats()
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.PendingCount != 1 {
		t.Fatalf("expected backlog count=1 for processing record, got %d", stats.PendingCount)
	}
}

func TestOutboxRepository_PullPendingReclaimsStaleProcessing(t *testing.T) {
	repo := NewOutboxRepository()

	saved, err := repo.Enqueue(domain.OutboxMessage{
		AggregateType: "order",
		AggregateID:   "order-stale",
		EventType:     "OrderCreated",
	})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	first, err := repo.PullPending(1)
	if err != nil {
		t.Fatalf("first pull failed: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected first pull to claim message, got %d", len(first))
	}

	repo.mu.Lock()
	record := repo.records[saved.ID]
	record.status = "processing"
	record.updatedAt = time.Now().UTC().Add(-outboxProcessingLease - time.Second)
	repo.mu.Unlock()

	reclaimed, err := repo.PullPending(1)
	if err != nil {
		t.Fatalf("reclaim pull failed: %v", err)
	}
	if len(reclaimed) != 1 {
		t.Fatalf("expected stale processing record to be reclaimed, got %d", len(reclaimed))
	}
	if reclaimed[0].ID != saved.ID {
		t.Fatalf("expected reclaimed id %s, got %s", saved.ID, reclaimed[0].ID)
	}
}
