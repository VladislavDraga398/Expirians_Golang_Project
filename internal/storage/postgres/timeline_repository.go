package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

type timelineRepository struct {
	db *sql.DB
}

// NewTimelineRepository создаёт PostgreSQL-реализацию TimelineRepository.
func NewTimelineRepository(store *Store) domain.TimelineRepository {
	return &timelineRepository{db: store.DB()}
}

func (r *timelineRepository) Append(event domain.TimelineEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	if event.Occurred.IsZero() {
		event.Occurred = time.Now().UTC()
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO timeline_events (order_id, type, reason, occurred)
		VALUES ($1,$2,$3,$4)
	`, event.OrderID, event.Type, event.Reason, event.Occurred); err != nil {
		return fmt.Errorf("append timeline event: %w", err)
	}

	return nil
}

func (r *timelineRepository) List(orderID string) ([]domain.TimelineEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT order_id, type, reason, occurred
		FROM timeline_events
		WHERE order_id = $1
		ORDER BY occurred ASC, id ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("list timeline events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.TimelineEvent, 0)
	for rows.Next() {
		var event domain.TimelineEvent
		if err := rows.Scan(&event.OrderID, &event.Type, &event.Reason, &event.Occurred); err != nil {
			return nil, fmt.Errorf("scan timeline event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeline events: %w", err)
	}

	return events, nil
}

var _ domain.TimelineRepository = (*timelineRepository)(nil)
