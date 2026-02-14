package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

type idempotencyRepositoryInMemory struct {
	mu    sync.RWMutex
	items map[string]domain.IdempotencyRecord
}

// NewIdempotencyRepository создаёт in-memory реализацию IdempotencyRepository.
func NewIdempotencyRepository() domain.IdempotencyRepository {
	return &idempotencyRepositoryInMemory{
		items: make(map[string]domain.IdempotencyRecord),
	}
}

func (r *idempotencyRepositoryInMemory) CreateProcessing(key, requestHash string, ttlAt time.Time) (domain.IdempotencyRecord, error) {
	key = strings.TrimSpace(key)
	requestHash = strings.TrimSpace(requestHash)

	if key == "" {
		return domain.IdempotencyRecord{}, domain.ErrIdempotencyKeyRequired
	}
	if requestHash == "" {
		return domain.IdempotencyRecord{}, domain.ErrIdempotencyRequestHashRequired
	}

	now := time.Now().UTC()
	if ttlAt.IsZero() {
		ttlAt = now.Add(24 * time.Hour)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.items[key]; ok {
		if existing.RequestHash != requestHash {
			return existing, domain.ErrIdempotencyHashMismatch
		}
		return existing, domain.ErrIdempotencyKeyAlreadyExists
	}

	record := domain.IdempotencyRecord{
		Key:          key,
		RequestHash:  requestHash,
		Status:       domain.IdempotencyStatusProcessing,
		TTLAt:        ttlAt,
		CreatedAt:    now,
		UpdatedAt:    now,
		ResponseBody: nil,
		HTTPStatus:   0,
	}

	r.items[key] = cloneIdempotencyRecord(record)
	return cloneIdempotencyRecord(record), nil
}

func (r *idempotencyRepositoryInMemory) Get(key string) (domain.IdempotencyRecord, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.IdempotencyRecord{}, domain.ErrIdempotencyKeyRequired
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.items[key]
	if !ok {
		return domain.IdempotencyRecord{}, domain.ErrIdempotencyKeyNotFound
	}

	return cloneIdempotencyRecord(record), nil
}

func (r *idempotencyRepositoryInMemory) MarkDone(key string, responseBody []byte, httpStatus int) error {
	return r.markStatus(key, domain.IdempotencyStatusDone, responseBody, httpStatus)
}

func (r *idempotencyRepositoryInMemory) MarkFailed(key string, responseBody []byte, httpStatus int) error {
	return r.markStatus(key, domain.IdempotencyStatusFailed, responseBody, httpStatus)
}

func (r *idempotencyRepositoryInMemory) DeleteExpired(before time.Time, limit int) (int, error) {
	if before.IsZero() {
		before = time.Now().UTC()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	for key, record := range r.items {
		if record.TTLAt.After(before) {
			continue
		}

		delete(r.items, key)
		removed++
		if limit > 0 && removed >= limit {
			break
		}
	}

	return removed, nil
}

func (r *idempotencyRepositoryInMemory) markStatus(key string, status domain.IdempotencyStatus, responseBody []byte, httpStatus int) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.ErrIdempotencyKeyRequired
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.items[key]
	if !ok {
		return domain.ErrIdempotencyKeyNotFound
	}

	record.Status = status
	record.ResponseBody = append([]byte(nil), responseBody...)
	record.HTTPStatus = httpStatus
	record.UpdatedAt = time.Now().UTC()
	r.items[key] = record

	return nil
}

func cloneIdempotencyRecord(src domain.IdempotencyRecord) domain.IdempotencyRecord {
	dst := src
	dst.ResponseBody = append([]byte(nil), src.ResponseBody...)
	return dst
}

var _ domain.IdempotencyRepository = (*idempotencyRepositoryInMemory)(nil)
