package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestWorker_ProcessOnce_MarkSent(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{
		pending: []domain.OutboxMessage{
			{
				ID:            "msg-1",
				AggregateType: "order",
				AggregateID:   "order-1",
				EventType:     "OrderStatusChanged",
				Payload:       []byte(`{"status":"confirmed"}`),
			},
		},
	}
	publisher := &stubPublisher{}

	worker := NewWorker(
		repo,
		publisher,
		WithRetryBaseDelay(0),
		WithMaxAttempts(3),
	)

	worker.ProcessOnce(context.Background())

	if got := len(repo.sentIDs); got != 1 {
		t.Fatalf("expected 1 sent mark, got %d", got)
	}
	if repo.sentIDs[0] != "msg-1" {
		t.Fatalf("expected sent id msg-1, got %s", repo.sentIDs[0])
	}
	if got := len(repo.failedIDs); got != 0 {
		t.Fatalf("expected 0 failed marks, got %d", got)
	}
	if got := publisher.calls(); got != 1 {
		t.Fatalf("expected 1 publish call, got %d", got)
	}
}

func TestWorker_ProcessOnce_MarkFailedAndDLQAfterRetries(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{
		pending: []domain.OutboxMessage{
			{
				ID:            "msg-2",
				AggregateType: "order",
				AggregateID:   "order-2",
				EventType:     "OrderStatusChanged",
				Payload:       []byte(`{"status":"canceled"}`),
			},
		},
	}
	publisher := &stubPublisher{err: errors.New("publish failed")}
	dlqPublisher := &stubPublisher{}

	worker := NewWorker(
		repo,
		publisher,
		WithDLQPublisher(dlqPublisher),
		WithRetryBaseDelay(0),
		WithMaxAttempts(3),
	)

	worker.ProcessOnce(context.Background())

	if got := publisher.calls(); got != 3 {
		t.Fatalf("expected 3 publish attempts, got %d", got)
	}
	if got := len(repo.sentIDs); got != 0 {
		t.Fatalf("expected 0 sent marks, got %d", got)
	}
	if got := len(repo.failedIDs); got != 1 {
		t.Fatalf("expected 1 failed mark, got %d", got)
	}
	if repo.failedIDs[0] != "msg-2" {
		t.Fatalf("expected failed id msg-2, got %s", repo.failedIDs[0])
	}
	if got := dlqPublisher.calls(); got != 1 {
		t.Fatalf("expected 1 DLQ publish, got %d", got)
	}
}

func TestWorker_ProcessOnce_SuccessAfterRetry(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{
		pending: []domain.OutboxMessage{
			{
				ID:            "msg-3",
				AggregateType: "order",
				AggregateID:   "order-3",
				EventType:     "OrderStatusChanged",
				Payload:       []byte(`{"status":"paid"}`),
			},
		},
	}
	publisher := &stubPublisher{
		sequenceErrors: []error{
			errors.New("attempt 1"),
			errors.New("attempt 2"),
			nil,
		},
	}

	worker := NewWorker(
		repo,
		publisher,
		WithRetryBaseDelay(0),
		WithMaxAttempts(3),
	)

	worker.ProcessOnce(context.Background())

	if got := publisher.calls(); got != 3 {
		t.Fatalf("expected 3 publish attempts, got %d", got)
	}
	if got := len(repo.sentIDs); got != 1 {
		t.Fatalf("expected 1 sent mark, got %d", got)
	}
	if got := len(repo.failedIDs); got != 0 {
		t.Fatalf("expected 0 failed marks, got %d", got)
	}
}

type stubOutboxRepo struct {
	pending   []domain.OutboxMessage
	sentIDs   []string
	failedIDs []string
}

func (s *stubOutboxRepo) Enqueue(msg domain.OutboxMessage) (domain.OutboxMessage, error) {
	return msg, nil
}

func (s *stubOutboxRepo) PullPending(limit int) ([]domain.OutboxMessage, error) {
	if limit <= 0 || limit >= len(s.pending) {
		return append([]domain.OutboxMessage(nil), s.pending...), nil
	}
	return append([]domain.OutboxMessage(nil), s.pending[:limit]...), nil
}

func (s *stubOutboxRepo) Stats() (domain.OutboxStats, error) {
	stats := domain.OutboxStats{
		PendingCount: len(s.pending),
	}
	if len(s.pending) > 0 {
		stats.OldestPendingAt = time.Now().UTC().Add(-time.Second)
	}
	return stats, nil
}

func (s *stubOutboxRepo) MarkSent(id string) error {
	s.sentIDs = append(s.sentIDs, id)
	return nil
}

func (s *stubOutboxRepo) MarkFailed(id string) error {
	s.failedIDs = append(s.failedIDs, id)
	return nil
}

type stubPublisher struct {
	mu             sync.Mutex
	err            error
	sequenceErrors []error
	callCount      int
}

func (s *stubPublisher) Publish(_ domain.OutboxMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callCount++
	if len(s.sequenceErrors) > 0 {
		err := s.sequenceErrors[0]
		s.sequenceErrors = s.sequenceErrors[1:]
		return err
	}

	return s.err
}

func (s *stubPublisher) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

var _ domain.OutboxRepository = (*stubOutboxRepo)(nil)
var _ domain.OutboxPublisher = (*stubPublisher)(nil)

func TestWorker_Run_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{}
	publisher := &stubPublisher{}

	worker := NewWorker(
		repo,
		publisher,
		WithPollInterval(5*time.Millisecond),
		WithRetryBaseDelay(0),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	time.Sleep(15 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not stop on context cancel")
	}
}
