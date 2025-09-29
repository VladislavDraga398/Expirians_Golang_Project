package domain

import "time"

// ReservationStatus отражает статус резервирования товара на складе.
type ReservationStatus string

const (
	// ReservationStatusPending — запрос на резервирование отправлен, ожидаем ответ склада.
	ReservationStatusPending ReservationStatus = "pending"
	// ReservationStatusReserved — товар успешно зарезервирован.
	ReservationStatusReserved ReservationStatus = "reserved"
	// ReservationStatusReleased — резерв снят (например, при отмене заказа).
	ReservationStatusReleased ReservationStatus = "released"
	// ReservationStatusFailed — резервирование не удалось.
	ReservationStatusFailed ReservationStatus = "failed"
)

// Reservation описывает конкретное резервирование товара под заказ.
type Reservation struct {
	ID        string
	OrderID   string
	SKU       string
	Qty       int32
	Status    ReservationStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate проверяет, корректно ли заполнены ключевые поля резервирования.
func (r *Reservation) Validate() []error {
	var errs []error

	if r.OrderID == "" {
		errs = append(errs, ErrOrderIDRequired)
	}
	if r.SKU == "" {
		errs = append(errs, ErrReservationSKURequired)
	}
	if r.Qty <= 0 {
		errs = append(errs, ErrReservationQtyInvalid)
	}

	return errs
}
