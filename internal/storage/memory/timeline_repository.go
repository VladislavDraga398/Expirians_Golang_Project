package memory

import (
	"sort"
	"sync"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

// timelineRepositoryInMemory хранит события в памяти (для разработки/тестов).
type timelineRepositoryInMemory struct {
	mu     sync.RWMutex
	events map[string][]domain.TimelineEvent
}

// NewTimelineRepository создаёт in-memory реализацию TimelineRepository.
func NewTimelineRepository() domain.TimelineRepository {
	return &timelineRepositoryInMemory{events: make(map[string][]domain.TimelineEvent)}
}

// Append добавляет событие в хранилище.
func (r *timelineRepositoryInMemory) Append(event domain.TimelineEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events[event.OrderID] = append(r.events[event.OrderID], event)

	sort.Slice(r.events[event.OrderID], func(i, j int) bool {
		return r.events[event.OrderID][i].Occurred.Before(r.events[event.OrderID][j].Occurred)
	})

	return nil
}

// List возвращает события заказа в хронологическом порядке.
func (r *timelineRepositoryInMemory) List(orderID string) ([]domain.TimelineEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := r.events[orderID]
	result := make([]domain.TimelineEvent, len(events))
	copy(result, events)
	return result, nil
}

var _ domain.TimelineRepository = (*timelineRepositoryInMemory)(nil)
