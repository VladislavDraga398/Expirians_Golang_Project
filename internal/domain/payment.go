package domain

import "time"

// PaymentStatus описывает состояние платежа в системе.
type PaymentStatus string

const (
	// PaymentStatusPending — платёж инициирован, но не подтверждён.
	PaymentStatusPending PaymentStatus = "pending"
	// PaymentStatusAuthorized — сумма успешно зарезервирована у провайдера.
	PaymentStatusAuthorized PaymentStatus = "authorized"
	// PaymentStatusCaptured — деньги списаны в пользу мерчанта.
	PaymentStatusCaptured PaymentStatus = "captured"
	// PaymentStatusRefunded — деньги возвращены клиенту полностью или частично.
	PaymentStatusRefunded PaymentStatus = "refunded"
	// PaymentStatusFailed — провайдер отклонил платёж или произошла ошибка.
	PaymentStatusFailed PaymentStatus = "failed"
)

// Payment описывает платёж, связанный с заказом.
type Payment struct {
	ID          string
	OrderID     string
	Provider    string
	ExternalID  string // Может быть пустым, если провайдер не возвращает идентификатор.
	Status      PaymentStatus
	AmountMinor int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate проверяет корректность полей платежа и возвращает ошибки, если они есть.
func (p *Payment) Validate() []error {
	var errs []error

	switch {
	case p.OrderID == "":
		errs = append(errs, ErrOrderIDRequired)
	case p.Provider == "":
		errs = append(errs, ErrPaymentProviderRequired)
	case p.AmountMinor < 0:
		errs = append(errs, ErrPaymentAmountNegative)
	}

	return errs
}
