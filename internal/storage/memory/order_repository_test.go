package memory_test

import (
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

func newOrder() domain.Order {
	now := time.Now().UTC()
	return domain.Order{
		ID:          "order-1",
		CustomerID:  "customer-1",
		Status:      domain.OrderStatusPending,
		Currency:    "USD",
		AmountMinor: 500,
		Items: []domain.OrderItem{
			{ID: "item-1", SKU: "sku-1", Qty: 5, PriceMinor: 100, CreatedAt: now},
		},
		Version:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestOrderRepository_CreateGet(t *testing.T) {
	repo := memory.NewOrderRepository()
	order := newOrder()

	if err := repo.Create(order); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	stored, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if stored.ID != order.ID {
		t.Fatalf("expected id %s, got %s", order.ID, stored.ID)
	}
}

func TestOrderRepository_ListByCustomer(t *testing.T) {
	repo := memory.NewOrderRepository()
	order := newOrder()
	if err := repo.Create(order); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	orders, err := repo.ListByCustomer(order.CustomerID, 10)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
}

func TestOrderRepository_Save(t *testing.T) {
	repo := memory.NewOrderRepository()
	order := newOrder()
	if err := repo.Create(order); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	stored, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	stored.AmountMinor = 600
	if err := repo.Save(stored); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	updated, err := repo.Get(order.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if updated.AmountMinor != 600 {
		t.Fatalf("expected amount 600, got %d", updated.AmountMinor)
	}
	if updated.Version != stored.Version+1 {
		t.Fatalf("expected version increment, got %d", updated.Version)
	}
}

func TestOrderRepository_SaveVersionConflict(t *testing.T) {
	repo := memory.NewOrderRepository()
	order := newOrder()
	if err := repo.Create(order); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	order.Version = 42
	if err := repo.Save(order); err == nil {
		t.Fatal("expected version conflict error")
	}
}
