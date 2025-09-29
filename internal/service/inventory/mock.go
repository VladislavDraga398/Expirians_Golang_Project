package inventory

import "github.com/vladislavdragonenkov/oms/internal/domain"

// MockService — конфигурируемая заглушка InventoryService для тестов.
type MockService struct {
	ReserveErr error
	ReleaseErr error

	ReserveCalls int
	ReleaseCalls int
}

// NewMockService возвращает mock с успешным сценарием по умолчанию.
func NewMockService() *MockService {
	return &MockService{}
}

// Reserve возвращает заранее настроенную ошибку и считает вызовы.
func (m *MockService) Reserve(orderID string, items []domain.OrderItem) error {
	m.ReserveCalls++
	return m.ReserveErr
}

// Release возвращает заранее настроенную ошибку и считает вызовы.
func (m *MockService) Release(orderID string, items []domain.OrderItem) error {
	m.ReleaseCalls++
	return m.ReleaseErr
}

var _ domain.InventoryService = (*MockService)(nil)
