package app

import (
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

// newTestOrder создаёт тестовый заказ для использования в тестах.
func newTestOrder() domain.Order {
	now := time.Now().UTC()
	return domain.Order{
		ID:          "test-order-1",
		CustomerID:  "test-customer-1",
		Status:      domain.OrderStatusPending,
		Currency:    "USD",
		AmountMinor: 1000,
		Items: []domain.OrderItem{
			{
				ID:         "item-1",
				SKU:        "SKU-TEST",
				Qty:        1,
				PriceMinor: 1000,
				CreatedAt:  now,
			},
		},
		Version:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
