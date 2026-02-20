package memory

import (
	"sort"
	"sync"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

// orderRepositoryInMemory — простая in-memory реализация OrderRepository.
type orderRepositoryInMemory struct {
	mu    sync.RWMutex
	items map[string]domain.Order
}

// NewOrderRepository возвращает in-memory репозиторий для локальной разработки и тестов.
func NewOrderRepository() domain.OrderRepository {
	return &orderRepositoryInMemory{
		items: make(map[string]domain.Order),
	}
}

// Create сохраняет новый заказ, если ID ещё не занят.
func (r *orderRepositoryInMemory) Create(order domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[order.ID]; exists {
		return domain.ErrOrderVersionConflict
	}
	// Сохраняем копию, чтобы избежать непредсказуемых мутаций извне.
	r.items[order.ID] = order
	return nil
}

// Get возвращает заказ или ErrOrderNotFound, если его нет.
func (r *orderRepositoryInMemory) Get(id string) (domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, ok := r.items[id]
	if !ok {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	return order, nil
}

// ListByCustomer возвращает заказы клиента, ограничивая выборку limit (если >0).
func (r *orderRepositoryInMemory) ListByCustomer(customerID string, limit int) ([]domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Order, 0, len(r.items))
	for _, order := range r.items {
		if order.CustomerID != customerID {
			continue
		}
		result = append(result, order)
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].ID > result[j].ID
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// Save перезаписывает заказ, проверяя версию (optimistic locking).
func (r *orderRepositoryInMemory) Save(order domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.items[order.ID]
	if !ok {
		return domain.ErrOrderNotFound
	}
	if current.Version != order.Version {
		return domain.ErrOrderVersionConflict
	}
	// Инкрементируем версию перед сохранением.
	order.Version++
	r.items[order.ID] = order
	return nil
}

var _ domain.OrderRepository = (*orderRepositoryInMemory)(nil)
