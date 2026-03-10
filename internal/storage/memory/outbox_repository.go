package memory

import (
	"sort"
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

const outboxProcessingLease = 2 * time.Minute

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

// PullPending атомарно claim'ит сообщения в обработку и возвращает до limit записей backlog.
// Также повторно выдаёт "зависшие" processing-записи после истечения lease.
func (r *outboxRepositoryInMemory) PullPending(limit int) ([]domain.OutboxMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limit <= 0 {
		limit = 100
	}

	now := time.Now().UTC()
	staleBefore := now.Add(-outboxProcessingLease)
	ids := make([]string, 0, len(r.records))
	for id := range r.records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left := r.records[ids[i]]
		right := r.records[ids[j]]
		if !left.createdAt.Equal(right.createdAt) {
			return left.createdAt.Before(right.createdAt)
		}
		return ids[i] < ids[j]
	})

	result := make([]domain.OutboxMessage, 0, limit)
	for _, id := range ids {
		rec := r.records[id]
		if rec.status != "pending" && (rec.status != "processing" || rec.updatedAt.After(staleBefore)) {
			continue
		}
		rec.status = "processing"
		rec.updatedAt = now
		result = append(result, rec.msg)
		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

// Stats возвращает сводную информацию о backlog outbox.
func (r *outboxRepositoryInMemory) Stats() (domain.OutboxStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := domain.OutboxStats{}
	for _, rec := range r.records {
		if rec.status != "pending" && rec.status != "processing" {
			continue
		}
		stats.PendingCount++
		if stats.OldestPendingAt.IsZero() || rec.createdAt.Before(stats.OldestPendingAt) {
			stats.OldestPendingAt = rec.createdAt
		}
	}

	return stats, nil
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
