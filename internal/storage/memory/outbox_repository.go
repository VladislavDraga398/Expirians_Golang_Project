package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

// outboxRecord хранит сообщение и служебные поля для in-memory реализации.
type outboxRecord struct {
	msg        domain.OutboxMessage
	status     string
	attemptCnt int
	createdAt  time.Time
	updatedAt  time.Time
}

// outboxRepositoryInMemory — простое in-memory хранилище для transactional outbox.
type outboxRepositoryInMemory struct {
	mu      sync.RWMutex
	records map[string]*outboxRecord
}

// NewOutboxRepository создаёт in-memory реализацию outbox.
func NewOutboxRepository() *outboxRepositoryInMemory {
	return &outboxRepositoryInMemory{records: make(map[string]*outboxRecord)}
}

// Enqueue сохраняет событие со статусом `pending` и возвращает его идентификатор.
func (r *outboxRepositoryInMemory) Enqueue(msg domain.OutboxMessage) (domain.OutboxMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	record := &outboxRecord{
		msg:       msg,
		status:    "pending",
		createdAt: now,
		updatedAt: now,
	}
	r.records[msg.ID] = record
	return msg, nil
}

// PullPending возвращает до limit сообщений со статусом `pending`.
func (r *outboxRepositoryInMemory) PullPending(limit int) []domain.OutboxMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	result := make([]domain.OutboxMessage, 0, limit)
	for _, rec := range r.records {
		if rec.status != "pending" {
			continue
		}
		result = append(result, rec.msg)
		if len(result) >= limit {
			break
		}
	}

	return result
}

// MarkSent обновляет статус события после успешной публикации.
func (r *outboxRepositoryInMemory) MarkSent(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[id]
	if !ok {
		return domain.ErrOutboxPublish
	}
	record.status = "sent"
	record.attemptCnt++
	record.updatedAt = time.Now().UTC()
	return nil
}

// MarkFailed фиксирует ошибку публикации.
func (r *outboxRepositoryInMemory) MarkFailed(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.records[id]
	if !ok {
		return domain.ErrOutboxPublish
	}
	record.status = "failed"
	record.attemptCnt++
	record.updatedAt = time.Now().UTC()
	return nil
}

// AllPending возвращает копию всех сообщений со статусом `pending` (используется в тестах).
func (r *outboxRepositoryInMemory) AllPending() []domain.OutboxMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.OutboxMessage, 0, len(r.records))
	for _, rec := range r.records {
		if rec.status == "pending" {
			result = append(result, rec.msg)
		}
	}
	return result
}

var _ domain.OutboxRepository = (*outboxRepositoryInMemory)(nil)
