package domain_test

import (
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

// helper для создания базового заказа с одной позицией.
func makeOrder() domain.Order {
	now := time.Now().UTC()
	return domain.Order{
		ID:          "order-1",
		CustomerID:  "customer-1",
		Status:      domain.OrderStatusPending,
		Currency:    "USD",
		AmountMinor: 500,
		Items: []domain.OrderItem{
			{
				ID:         "item-1",
				SKU:        "sku-1",
				Qty:        5,
				PriceMinor: 100,
				CreatedAt:  now,
			},
		},
		Version:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestOrderValidateInvariants_Ok(t *testing.T) {
	order := makeOrder()
	if errs := order.ValidateInvariants(); len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}
}

func TestOrderValidateInvariants_Errors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(o *domain.Order)
	}{
		{
			name: "no customer",
			mut: func(o *domain.Order) {
				o.CustomerID = ""
			},
		},
		{
			name: "negative amount",
			mut: func(o *domain.Order) {
				o.AmountMinor = -1
			},
		},
		{
			name: "no items",
			mut: func(o *domain.Order) {
				o.Items = nil
			},
		},
		{
			name: "qty invalid",
			mut: func(o *domain.Order) {
				o.Items[0].Qty = 0
			},
		},
		{
			name: "price invalid",
			mut: func(o *domain.Order) {
				o.Items[0].PriceMinor = -5
			},
		},
		{
			name: "amount mismatch",
			mut: func(o *domain.Order) {
				o.AmountMinor = 999
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order := makeOrder()
			if len(order.Items) == 0 {
				t.Fatal("test setup produced order without items")
			}
			// Изменяем состояние согласно сценарию.
			mutOrder := order
			tc.mut(&mutOrder)

			if len(mutOrder.ValidateInvariants()) == 0 {
				t.Fatalf("expected validation errors for case %s", tc.name)
			}
		})
	}
}
