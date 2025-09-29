package payment

import "github.com/vladislavdragonenkov/oms/internal/domain"

// MockService — конфигурируемая заглушка PaymentService для тестов.
type MockService struct {
	PayStatus    domain.PaymentStatus
	PayErr       error
	RefundStatus domain.PaymentStatus
	RefundErr    error

	PayCalls    int
	RefundCalls int
}

// NewMockService возвращает mock с успешным сценарием по умолчанию.
func NewMockService() *MockService {
	return &MockService{
		PayStatus:    domain.PaymentStatusCaptured,
		RefundStatus: domain.PaymentStatusRefunded,
	}
}

// Pay возвращает заранее настроенный результат и считает вызовы.
func (m *MockService) Pay(orderID string, amountMinor int64, currency string) (domain.PaymentStatus, error) {
	m.PayCalls++
	return m.PayStatus, m.PayErr
}

// Refund возвращает настроенный результат и считает вызовы.
func (m *MockService) Refund(orderID string, amountMinor int64, currency string) (domain.PaymentStatus, error) {
	m.RefundCalls++
	return m.RefundStatus, m.RefundErr
}

var _ domain.PaymentService = (*MockService)(nil)
