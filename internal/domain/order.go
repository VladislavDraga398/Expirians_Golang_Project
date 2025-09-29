package domain

import "time"

// OrderStatus описывает жизненный цикл заказа в OMS.
type OrderStatus string

const (
	// OrderStatusPending — заказ создан, но резервирование и оплата ещё не выполнены.
	OrderStatusPending OrderStatus = "pending"
	// OrderStatusReserved — товары зарезервированы на складе.
	OrderStatusReserved OrderStatus = "reserved"
	// OrderStatusPaid — оплата подтверждена платёжным провайдером.
	OrderStatusPaid OrderStatus = "paid"
	// OrderStatusConfirmed — заказ финализирован и готов к исполнению.
	OrderStatusConfirmed OrderStatus = "confirmed"
	// OrderStatusCanceled — заказ отменён до завершения цикла.
	OrderStatusCanceled OrderStatus = "canceled"
	// OrderStatusRefunded — заказ полностью или частично возвращён клиенту.
	OrderStatusRefunded OrderStatus = "refunded"
)

// OrderItem представляет одну позицию заказа.
type OrderItem struct {
	// ID позиции нужен для однозначной идентификации и аудита.
	ID string
	// SKU — внешний идентификатор товара.
	SKU string
	// Qty — количество единиц товара.
	Qty int32
	// PriceMinor — цена за единицу в минимальных денежных единицах (например, копейки).
	PriceMinor int64
	// CreatedAt фиксирует момент добавления позиции в заказ.
	CreatedAt time.Time
}

// Order агрегирует состояние заказа и его позиции.
type Order struct {
	ID          string
	CustomerID  string
	Status      OrderStatus
	Currency    string
	AmountMinor int64
	Items       []OrderItem
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateInvariants проверяет базовые инварианты заказа и возвращает список замечаний.
func (o *Order) ValidateInvariants() []error {
	var errs []error

	if o.CustomerID == "" {
		errs = append(errs, ErrCustomerRequired)
	}
	if o.Currency == "" {
		errs = append(errs, ErrCurrencyRequired)
	}
	if len(o.Items) == 0 {
		errs = append(errs, ErrItemsRequired)
	}
	if o.AmountMinor < 0 {
		errs = append(errs, ErrAmountNegative)
	}

	// Сверяем сумму заказа с суммой позиций: qty * price.
	var calc int64
	for _, item := range o.Items {
		if item.Qty <= 0 {
			errs = append(errs, ErrItemQtyInvalid)
		}
		if item.PriceMinor < 0 {
			errs = append(errs, ErrItemPriceInvalid)
		}
		calc += int64(item.Qty) * item.PriceMinor
	}
	if calc != o.AmountMinor {
		errs = append(errs, ErrAmountMismatch)
	}

	return errs
}
