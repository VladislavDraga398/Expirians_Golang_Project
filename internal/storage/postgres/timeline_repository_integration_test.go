package postgres

import (
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestTimelineRepository_PostgresAppendAndList(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	orderRepo := NewOrderRepository(store)
	timelineRepo := NewTimelineRepository(store)

	createdAt := time.Now().UTC().Add(-time.Minute).Round(time.Microsecond)
	order := sampleOrder("timeline-order", "customer-timeline", createdAt)
	if err := orderRepo.Create(order); err != nil {
		t.Fatalf("create order for timeline: %v", err)
	}

	// Zero occurred should be auto-filled.
	if err := timelineRepo.Append(domain.TimelineEvent{
		OrderID: order.ID,
		Type:    "OrderCreated",
		Reason:  "created",
	}); err != nil {
		t.Fatalf("append timeline event with zero occurred: %v", err)
	}

	explicitOccurred := createdAt.Add(10 * time.Second)
	if err := timelineRepo.Append(domain.TimelineEvent{
		OrderID:  order.ID,
		Type:     "OrderPaid",
		Reason:   "paid",
		Occurred: explicitOccurred,
	}); err != nil {
		t.Fatalf("append timeline event with explicit occurred: %v", err)
	}

	events, err := timelineRepo.List(order.ID)
	if err != nil {
		t.Fatalf("list timeline events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 timeline events, got %d", len(events))
	}
	if events[0].Occurred.After(events[1].Occurred) {
		t.Fatalf("events should be sorted by occurred asc: %+v", events)
	}
	types := []string{events[0].Type, events[1].Type}
	if !(contains(types, "OrderCreated") && contains(types, "OrderPaid")) {
		t.Fatalf("unexpected event types: %+v", types)
	}
}

func TestTimelineRepository_PostgresMissingOrder(t *testing.T) {
	store := openPostgresStoreForIntegrationTest(t)
	timelineRepo := NewTimelineRepository(store)

	if err := timelineRepo.Append(domain.TimelineEvent{
		OrderID: "missing-order",
		Type:    "OrderCreated",
		Reason:  "test",
	}); err == nil {
		t.Fatal("expected append error for missing order due FK constraint")
	}

	events, err := timelineRepo.List("missing-order")
	if err != nil {
		t.Fatalf("list for missing order should not fail: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events for missing order, got %d", len(events))
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
