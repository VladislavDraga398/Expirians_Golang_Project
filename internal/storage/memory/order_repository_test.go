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

func TestOrderRepository_ListByCustomer_SortedAndLimited(t *testing.T) {
	repo := memory.NewOrderRepository()
	base := time.Now().UTC()

	orders := []domain.Order{
		{
			ID:          "order-1",
			CustomerID:  "customer-1",
			Status:      domain.OrderStatusPending,
			Currency:    "USD",
			AmountMinor: 100,
			Items:       []domain.OrderItem{{ID: "item-1", SKU: "sku-1", Qty: 1, PriceMinor: 100, CreatedAt: base}},
			CreatedAt:   base.Add(-2 * time.Minute),
			UpdatedAt:   base.Add(-2 * time.Minute),
		},
		{
			ID:          "order-3",
			CustomerID:  "customer-1",
			Status:      domain.OrderStatusPending,
			Currency:    "USD",
			AmountMinor: 100,
			Items:       []domain.OrderItem{{ID: "item-3", SKU: "sku-3", Qty: 1, PriceMinor: 100, CreatedAt: base}},
			CreatedAt:   base,
			UpdatedAt:   base,
		},
		{
			ID:          "order-2",
			CustomerID:  "customer-1",
			Status:      domain.OrderStatusPending,
			Currency:    "USD",
			AmountMinor: 100,
			Items:       []domain.OrderItem{{ID: "item-2", SKU: "sku-2", Qty: 1, PriceMinor: 100, CreatedAt: base}},
			CreatedAt:   base,
			UpdatedAt:   base,
		},
	}

	for _, order := range orders {
		if err := repo.Create(order); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	listed, err := repo.ListByCustomer("customer-1", 2)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 orders with limit, got %d", len(listed))
	}
	if listed[0].ID != "order-3" {
		t.Fatalf("expected newest order with max ID first, got %s", listed[0].ID)
	}
	if listed[1].ID != "order-2" {
		t.Fatalf("expected second newest order, got %s", listed[1].ID)
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
