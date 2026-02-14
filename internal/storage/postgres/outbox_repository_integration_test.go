package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestOutboxRepository_PostgresFlow(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewOutboxRepository(store)

	msgWithoutID := domain.OutboxMessage{
		AggregateType: "order",
		AggregateID:   "order-1",
		EventType:     "OrderCreated",
		Payload:       []byte(`{"id":"order-1"}`),
	}
	stored1, err := repo.Enqueue(msgWithoutID)
	if err != nil {
		t.Fatalf("enqueue msg without id: %v", err)
	}
	if stored1.ID == "" {
		t.Fatal("expected generated id for outbox message")
	}

	msgWithID := domain.OutboxMessage{
		ID:            "outbox-fixed-id",
		AggregateType: "order",
		AggregateID:   "order-2",
		EventType:     "OrderUpdated",
		Payload:       []byte(`{"id":"order-2"}`),
	}
	stored2, err := repo.Enqueue(msgWithID)
	if err != nil {
		t.Fatalf("enqueue msg with id: %v", err)
	}
	if stored2.ID != msgWithID.ID {
		t.Fatalf("expected fixed id %q, got %q", msgWithID.ID, stored2.ID)
	}

	pending, err := repo.PullPending(0) // default limit path
	if err != nil {
		t.Fatalf("pull pending: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending messages, got %d", len(pending))
	}

	stats, err := repo.Stats()
	if err != nil {
		t.Fatalf("stats before marks: %v", err)
	}
	if stats.PendingCount != 2 {
		t.Fatalf("expected pending=2 before marks, got %d", stats.PendingCount)
	}
	if stats.OldestPendingAt.IsZero() {
		t.Fatal("expected oldest pending timestamp")
	}

	if err := repo.MarkSent(stored1.ID); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if err := repo.MarkFailed(stored2.ID); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	after, err := repo.PullPending(10)
	if err != nil {
		t.Fatalf("pull pending after marks: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("expected no pending after marks, got %d", len(after))
	}

	stats, err = repo.Stats()
	if err != nil {
		t.Fatalf("stats after marks: %v", err)
	}
	if stats.PendingCount != 0 {
		t.Fatalf("expected pending=0 after marks, got %d", stats.PendingCount)
	}
}

func TestOutboxRepository_PostgresMissingRows(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewOutboxRepository(store)

	if err := repo.MarkSent("missing-outbox"); !errors.Is(err, domain.ErrOutboxPublish) {
		t.Fatalf("expected ErrOutboxPublish on mark sent missing id, got %v", err)
	}
	if err := repo.MarkFailed("missing-outbox"); !errors.Is(err, domain.ErrOutboxPublish) {
		t.Fatalf("expected ErrOutboxPublish on mark failed missing id, got %v", err)
	}
}

func TestOutboxRepository_PostgresStatsOldestPendingOrder(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	repo := NewOutboxRepository(store)

	first, err := repo.Enqueue(domain.OutboxMessage{
		AggregateType: "order",
		AggregateID:   "order-old",
		EventType:     "OrderCreated",
		Payload:       []byte(`{"id":"order-old"}`),
	})
	if err != nil {
		t.Fatalf("enqueue first: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	if _, err := repo.Enqueue(domain.OutboxMessage{
		AggregateType: "order",
		AggregateID:   "order-new",
		EventType:     "OrderCreated",
		Payload:       []byte(`{"id":"order-new"}`),
	}); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	stats, err := repo.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.PendingCount != 2 {
		t.Fatalf("expected pending=2, got %d", stats.PendingCount)
	}
	if stats.OldestPendingAt.IsZero() {
		t.Fatal("expected non-zero oldest pending time")
	}

	if err := repo.MarkSent(first.ID); err != nil {
		t.Fatalf("mark sent first: %v", err)
	}
}
