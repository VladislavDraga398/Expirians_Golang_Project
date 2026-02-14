package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

type idempotencyRepository struct {
	db *sql.DB
}

// NewIdempotencyRepository создаёт PostgreSQL-реализацию IdempotencyRepository.
func NewIdempotencyRepository(store *Store) domain.IdempotencyRepository {
	return &idempotencyRepository{db: store.DB()}
}

func (r *idempotencyRepository) CreateProcessing(key, requestHash string, ttlAt time.Time) (domain.IdempotencyRecord, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (
			key, request_hash, response_body, http_status, status, ttl_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		key,
		requestHash,
		nil,
		nil,
		string(domain.IdempotencyStatusProcessing),
		ttlAt,
		now,
		now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			existing, getErr := r.Get(key)
			if getErr != nil {
				return domain.IdempotencyRecord{}, domain.ErrIdempotencyKeyAlreadyExists
			}
			if existing.RequestHash != requestHash {
				return existing, domain.ErrIdempotencyHashMismatch
			}
			return existing, domain.ErrIdempotencyKeyAlreadyExists
		}
		return domain.IdempotencyRecord{}, fmt.Errorf("create idempotency record: %w", err)
	}

	return domain.IdempotencyRecord{
		Key:          key,
		RequestHash:  requestHash,
		Status:       domain.IdempotencyStatusProcessing,
		TTLAt:        ttlAt,
		CreatedAt:    now,
		UpdatedAt:    now,
		ResponseBody: nil,
		HTTPStatus:   0,
	}, nil
}

func (r *idempotencyRepository) Get(key string) (domain.IdempotencyRecord, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.IdempotencyRecord{}, domain.ErrIdempotencyKeyRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	var (
		record       domain.IdempotencyRecord
		statusRaw    string
		responseBody []byte
		httpStatus   sql.NullInt64
	)

	err := r.db.QueryRowContext(ctx, `
		SELECT key, request_hash, response_body, http_status, status, ttl_at, created_at, updated_at
		FROM idempotency_keys
		WHERE key = $1
	`, key).Scan(
		&record.Key,
		&record.RequestHash,
		&responseBody,
		&httpStatus,
		&statusRaw,
		&record.TTLAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.IdempotencyRecord{}, domain.ErrIdempotencyKeyNotFound
		}
		return domain.IdempotencyRecord{}, fmt.Errorf("get idempotency record: %w", err)
	}

	record.Status = domain.IdempotencyStatus(statusRaw)
	if !record.Status.Valid() {
		return domain.IdempotencyRecord{}, fmt.Errorf("invalid idempotency status %q for key %s", statusRaw, key)
	}

	record.ResponseBody = append([]byte(nil), responseBody...)
	if httpStatus.Valid {
		record.HTTPStatus = int(httpStatus.Int64)
	}

	return record, nil
}

func (r *idempotencyRepository) MarkDone(key string, responseBody []byte, httpStatus int) error {
	return r.markStatus(key, domain.IdempotencyStatusDone, responseBody, httpStatus)
}

func (r *idempotencyRepository) MarkFailed(key string, responseBody []byte, httpStatus int) error {
	return r.markStatus(key, domain.IdempotencyStatusFailed, responseBody, httpStatus)
}

func (r *idempotencyRepository) DeleteExpired(before time.Time, limit int) (int, error) {
	if before.IsZero() {
		before = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	var (
		res sql.Result
		err error
	)

	if limit > 0 {
		res, err = r.db.ExecContext(ctx, `
			DELETE FROM idempotency_keys
			WHERE key IN (
				SELECT key
				FROM idempotency_keys
				WHERE ttl_at <= $1
				ORDER BY ttl_at ASC
				LIMIT $2
			)
		`, before, limit)
	} else {
		res, err = r.db.ExecContext(ctx, `
			DELETE FROM idempotency_keys
			WHERE ttl_at <= $1
		`, before)
	}
	if err != nil {
		return 0, fmt.Errorf("delete expired idempotency records: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("idempotency rows affected: %w", err)
	}

	return int(affected), nil
}

func (r *idempotencyRepository) markStatus(key string, status domain.IdempotencyStatus, responseBody []byte, httpStatus int) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.ErrIdempotencyKeyRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET response_body = $1,
		    http_status = $2,
		    status = $3,
		    updated_at = $4
		WHERE key = $5
	`,
		responseBody,
		httpStatus,
		string(status),
		time.Now().UTC(),
		key,
	)
	if err != nil {
		return fmt.Errorf("mark idempotency key status: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("idempotency rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrIdempotencyKeyNotFound
	}

	return nil
}

var _ domain.IdempotencyRepository = (*idempotencyRepository)(nil)
