package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

type outboxRepository struct {
	db *sql.DB
}

// NewOutboxRepository создаёт PostgreSQL-реализацию OutboxRepository.
func NewOutboxRepository(store *Store) domain.OutboxRepository {
	return &outboxRepository{db: store.DB()}
}

func (r *outboxRepository) Enqueue(msg domain.OutboxMessage) (domain.OutboxMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	now := time.Now().UTC()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox_messages (
			id, aggregate_type, aggregate_id, event_type, payload,
			status, attempt_count, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,'pending',0,$6,$7)
	`,
		msg.ID, msg.AggregateType, msg.AggregateID, msg.EventType, msg.Payload, now, now,
	)
	if err != nil {
		return domain.OutboxMessage{}, fmt.Errorf("enqueue outbox message: %w", err)
	}

	return msg, nil
}

func (r *outboxRepository) PullPending(limit int) ([]domain.OutboxMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, aggregate_type, aggregate_id, event_type, payload
		FROM outbox_messages
		WHERE status = 'pending'
		ORDER BY created_at, id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("pull pending outbox messages: %w", err)
	}
	defer rows.Close()

	result := make([]domain.OutboxMessage, 0, limit)
	for rows.Next() {
		var msg domain.OutboxMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.AggregateType,
			&msg.AggregateID,
			&msg.EventType,
			&msg.Payload,
		); err != nil {
			return nil, fmt.Errorf("scan outbox message: %w", err)
		}
		result = append(result, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox rows: %w", err)
	}

	return result, nil
}

func (r *outboxRepository) Stats() (domain.OutboxStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	var (
		stats  domain.OutboxStats
		oldest sql.NullTime
	)

	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(created_at)
		FROM outbox_messages
		WHERE status = 'pending'
	`).Scan(&stats.PendingCount, &oldest); err != nil {
		return domain.OutboxStats{}, fmt.Errorf("outbox stats query failed: %w", err)
	}

	if oldest.Valid {
		stats.OldestPendingAt = oldest.Time.UTC()
	}

	return stats, nil
}

func (r *outboxRepository) MarkSent(id string) error {
	return r.markStatus(id, "sent")
}

func (r *outboxRepository) MarkFailed(id string) error {
	return r.markStatus(id, "failed")
}

func (r *outboxRepository) markStatus(id, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `
		UPDATE outbox_messages
		SET status = $2,
		    attempt_count = attempt_count + 1,
		    updated_at = $3
		WHERE id = $1
	`, id, status, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("mark outbox message as %s: %w", status, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for outbox %s: %w", status, err)
	}
	if affected == 0 {
		return domain.ErrOutboxPublish
	}

	return nil
}

var _ domain.OutboxRepository = (*outboxRepository)(nil)
