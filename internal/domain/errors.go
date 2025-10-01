package domain

import "errors"

var (
	// Ошибка отсутствующего идентификатора клиента.
	ErrCustomerRequired = errors.New("customer_id is required")
	// Ошибка отсутствующего кода валюты.
	ErrCurrencyRequired = errors.New("currency is required")
	// Ошибка отсутствия хотя бы одного товара в заказе.
	ErrItemsRequired = errors.New("order must contain at least one item")
	// Ошибка отрицательной суммы заказа.
	ErrAmountNegative = errors.New("amount_minor must be non-negative")
	// Ошибка при некорректном количестве товара (<= 0).
	ErrItemQtyInvalid = errors.New("item qty must be greater than zero")
	// Ошибка, если цена позиции отрицательная.
	ErrItemPriceInvalid = errors.New("item price must be non-negative")
	// Ошибка несоответствия суммы заказа и сумм позиций.
	ErrAmountMismatch = errors.New("order amount does not match items sum")
	// Ошибка отрицательной суммы платежа.
	ErrPaymentAmountNegative = errors.New("payment amount must be non-negative")
	// Ошибка отсутствующего кода платёжного провайдера.
	ErrPaymentProviderRequired = errors.New("payment provider is required")
	// Ошибка отсутствующего идентификатора заказа в платежах/резервах.
	ErrOrderIDRequired = errors.New("order_id is required")
	// Ошибка отсутствующего SKU в резерве.
	ErrReservationSKURequired = errors.New("reservation sku is required")
	// Ошибка некорректного количества в резерве.
	ErrReservationQtyInvalid = errors.New("reservation qty must be greater than zero")
	// ErrOrderNotFound возвращается, если заказ не найден в репозитории.
	ErrOrderNotFound = errors.New("order not found")
	// ErrOrderVersionConflict сигнализирует о конфликте версий при сохранении.
	ErrOrderVersionConflict = errors.New("order version conflict")
	// ErrInventoryUnavailable — бизнес-ошибка от склада (нет стока/недоступность позиции).
	ErrInventoryUnavailable = errors.New("inventory unavailable")
	// ErrInventoryTemporary — временная ошибка при обращении к складу, можно повторить попытку.
	ErrInventoryTemporary = errors.New("inventory temporary error")
	// ErrPaymentDeclined — платёж отклонён провайдером (бизнес-ошибка).
	ErrPaymentDeclined = errors.New("payment declined")
	// ErrPaymentIndeterminate — неопределённый статус платежа; требуется reconcile.
	ErrPaymentIndeterminate = errors.New("payment indeterminate state")
	// ErrPaymentTemporary — временная ошибка платёжного провайдера.
	ErrPaymentTemporary = errors.New("payment temporary error")
	// ErrOutboxPublish — ошибка при публикации сообщения из outbox.
	ErrOutboxPublish = errors.New("outbox publish failed")
)

// IsVersionConflict проверяет, является ли ошибка конфликтом версий.
func IsVersionConflict(err error) bool {
	return errors.Is(err, ErrOrderVersionConflict)
}
