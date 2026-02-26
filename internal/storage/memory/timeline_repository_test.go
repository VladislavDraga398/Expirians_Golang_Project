package memory

import (
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestTimelineRepository_AppendAndListSorted(t *testing.T) {
	repo := NewTimelineRepository()

	now := time.Now().UTC().Round(time.Microsecond)
	events := []domain.TimelineEvent{
		{OrderID: "order-1", Type: "paid", Occurred: now.Add(10 * time.Second)},
		{OrderID: "order-1", Type: "created", Occurred: now},
		{OrderID: "order-1", Type: "confirmed", Occurred: now.Add(20 * time.Second)},
	}

	for _, event := range events {
		if err := repo.Append(event); err != nil {
			t.Fatalf("append event failed: %v", err)
		}
	}

	listed, err := repo.List("order-1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 events, got %d", len(listed))
	}
	if listed[0].Type != "created" || listed[1].Type != "paid" || listed[2].Type != "confirmed" {
		t.Fatalf("unexpected event order: %+v", listed)
	}
}

func TestTimelineRepository_ListReturnsCopyAndMissingOrder(t *testing.T) {
	repo := NewTimelineRepository()

	now := time.Now().UTC().Round(time.Microsecond)
	event := domain.TimelineEvent{OrderID: "order-copy", Type: "created", Occurred: now}
	if err := repo.Append(event); err != nil {
		t.Fatalf("append event failed: %v", err)
	}

	firstRead, err := repo.List("order-copy")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(firstRead) != 1 {
		t.Fatalf("expected single event, got %d", len(firstRead))
	}

	firstRead[0].Type = "mutated"

	secondRead, err := repo.List("order-copy")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if secondRead[0].Type != "created" {
		t.Fatalf("repository must return copy, got %+v", secondRead[0])
	}

	empty, err := repo.List("missing-order")
	if err != nil {
		t.Fatalf("list missing order failed: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no events for missing order, got %d", len(empty))
	}
}
