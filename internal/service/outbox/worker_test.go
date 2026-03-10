package outbox

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

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

func TestNewWorker_OptionsAndNormalization(t *testing.T) {
	t.Parallel()

	logger := log.WithField("test", "outbox-worker")
	worker := NewWorker(
		&stubOutboxRepo{},
		&stubPublisher{},
		WithLogger(logger),
		WithBatchSize(7),
		WithPollInterval(0),
		WithMaxAttempts(0),
		WithRetryBaseDelay(-time.Millisecond),
	)

	if worker.logger != logger {
		t.Fatal("expected custom logger to be used")
	}
	if worker.batchSize != 7 {
		t.Fatalf("expected batch size 7, got %d", worker.batchSize)
	}
	if worker.pollInterval != defaultPollInterval {
		t.Fatalf("expected default poll interval %s, got %s", defaultPollInterval, worker.pollInterval)
	}
	if worker.maxAttempts != defaultMaxAttempts {
		t.Fatalf("expected default max attempts %d, got %d", defaultMaxAttempts, worker.maxAttempts)
	}
	if worker.retryBaseDelay != 0 {
		t.Fatalf("expected retry base delay 0 after normalization, got %s", worker.retryBaseDelay)
	}
}

func TestWorker_Run_DisabledWhenDependenciesMissing(t *testing.T) {
	t.Parallel()

	runAndWait := func(worker *Worker) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			defer close(done)
			worker.Run(context.Background())
		}()

		select {
		case <-done:
		case <-time.After(300 * time.Millisecond):
			t.Fatal("worker.Run should return immediately when dependencies are missing")
		}
	}

	runAndWait(NewWorker(nil, &stubPublisher{}))
	runAndWait(NewWorker(&stubOutboxRepo{}, nil))
}

func TestWorker_ProcessOnce_PullPendingError(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{pullErr: errors.New("pull failed")}
	publisher := &stubPublisher{}
	worker := NewWorker(repo, publisher)

	worker.ProcessOnce(context.Background())

	if repo.pullCalls != 1 {
		t.Fatalf("expected 1 pull call, got %d", repo.pullCalls)
	}
	if got := publisher.calls(); got != 0 {
		t.Fatalf("expected no publish calls on pull error, got %d", got)
	}
}

func TestWorker_ProcessOnce_ContextCanceledNoop(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{}
	publisher := &stubPublisher{}
	worker := NewWorker(repo, publisher)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker.ProcessOnce(ctx)

	if repo.statsCalls != 0 || repo.pullCalls != 0 {
		t.Fatalf("expected no repository calls for canceled context, stats=%d pull=%d", repo.statsCalls, repo.pullCalls)
	}
}

func TestWorker_ProcessOnce_MarkSentErrorPath(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{
		pending: []domain.OutboxMessage{
			{
				ID:            "msg-mark-sent",
				AggregateType: "order",
				AggregateID:   "order-1",
				EventType:     "OrderStatusChanged",
				Payload:       []byte(`{"status":"confirmed"}`),
			},
		},
		markSentErr: errors.New("mark sent failed"),
	}
	publisher := &stubPublisher{}
	worker := NewWorker(repo, publisher, WithRetryBaseDelay(0))

	worker.ProcessOnce(context.Background())

	if got := len(repo.sentIDs); got != 1 {
		t.Fatalf("expected mark sent to be attempted once, got %d", got)
	}
	if got := len(repo.failedIDs); got != 0 {
		t.Fatalf("expected no failed marks, got %d", got)
	}
}

func TestWorker_ProcessOnce_DLQAndMarkFailedErrorPaths(t *testing.T) {
	t.Parallel()

	repo := &stubOutboxRepo{
		pending: []domain.OutboxMessage{
			{
				ID:            "msg-dlq",
				AggregateType: "order",
				AggregateID:   "order-2",
				EventType:     "OrderStatusChanged",
				Payload:       []byte(`{"status":"canceled"}`),
			},
		},
		markFailedErr: errors.New("mark failed error"),
	}
	publisher := &stubPublisher{err: errors.New("publish error")}
	dlqPublisher := &stubPublisher{err: errors.New("dlq publish error")}
	worker := NewWorker(
		repo,
		publisher,
		WithDLQPublisher(dlqPublisher),
		WithRetryBaseDelay(0),
		WithMaxAttempts(2),
	)

	worker.ProcessOnce(context.Background())

	if got := publisher.calls(); got != 2 {
		t.Fatalf("expected 2 publish attempts, got %d", got)
	}
	if got := dlqPublisher.calls(); got != 1 {
		t.Fatalf("expected 1 dlq publish attempt, got %d", got)
	}
	if got := len(repo.failedIDs); got != 1 {
		t.Fatalf("expected 1 mark failed call, got %d", got)
	}
}

func TestWorker_PublishToDLQ_EdgeCases(t *testing.T) {
	t.Parallel()

	event := domain.OutboxMessage{
		ID:            "msg-dlq-case",
		AggregateType: "order",
		AggregateID:   "order-3",
		EventType:     "OrderStatusChanged",
		Payload:       []byte(`{"ok":true}`),
	}

	workerNoDLQ := NewWorker(&stubOutboxRepo{}, &stubPublisher{})
	if err := workerNoDLQ.publishToDLQ(event, errors.New("publish failed")); err != nil {
		t.Fatalf("expected nil error when dlq publisher is not configured, got %v", err)
	}

	workerMarshal := NewWorker(
		&stubOutboxRepo{},
		&stubPublisher{},
		WithDLQPublisher(&stubPublisher{}),
	)
	badPayloadEvent := event
	badPayloadEvent.Payload = []byte(`{invalid-json`)
	err := workerMarshal.publishToDLQ(badPayloadEvent, errors.New("publish failed"))
	if err == nil || !strings.Contains(err.Error(), "marshal dlq payload") {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestWorker_RetryBackoff(t *testing.T) {
	t.Parallel()

	noDelayWorker := NewWorker(&stubOutboxRepo{}, &stubPublisher{}, WithRetryBaseDelay(0))
	if got := noDelayWorker.retryBackoff(3); got != 0 {
		t.Fatalf("expected zero delay when retry base delay is 0, got %s", got)
	}

	worker := NewWorker(&stubOutboxRepo{}, &stubPublisher{}, WithRetryBaseDelay(10*time.Millisecond))
	if got := worker.retryBackoff(1); got != 10*time.Millisecond {
		t.Fatalf("expected first attempt delay to equal base delay, got %s", got)
	}
	if got := worker.retryBackoff(3); got != 40*time.Millisecond {
		t.Fatalf("expected exponential delay 40ms, got %s", got)
	}

	const maxDuration = time.Duration(1<<63 - 1)
	overflowWorker := NewWorker(
		&stubOutboxRepo{},
		&stubPublisher{},
		WithRetryBaseDelay(maxDuration/2+1),
	)
	if got := overflowWorker.retryBackoff(2); got != maxDuration {
		t.Fatalf("expected overflow guard to cap delay at max duration, got %s", got)
	}
}

func TestWorker_RefreshBacklogMetrics_StatsErrorAndFutureTimestamp(t *testing.T) {
	t.Parallel()

	errorRepo := &stubOutboxRepo{statsErr: errors.New("stats error")}
	errorWorker := NewWorker(errorRepo, &stubPublisher{})
	errorWorker.refreshBacklogMetrics()
	if errorRepo.statsCalls != 1 {
		t.Fatalf("expected stats to be called once on error path, got %d", errorRepo.statsCalls)
	}

	futureRepo := &stubOutboxRepo{
		stats: domain.OutboxStats{
			PendingCount:    1,
			OldestPendingAt: time.Now().UTC().Add(5 * time.Second),
		},
	}
	futureWorker := NewWorker(futureRepo, &stubPublisher{})
	futureWorker.refreshBacklogMetrics()
	if futureRepo.statsCalls != 1 {
		t.Fatalf("expected stats to be called once for future timestamp path, got %d", futureRepo.statsCalls)
	}
}

type stubOutboxRepo struct {
	pending       []domain.OutboxMessage
	sentIDs       []string
	failedIDs     []string
	pullErr       error
	stats         domain.OutboxStats
	statsErr      error
	markSentErr   error
	markFailedErr error
	pullCalls     int
	statsCalls    int
}

func (s *stubOutboxRepo) Enqueue(msg domain.OutboxMessage) (domain.OutboxMessage, error) {
	return msg, nil
}

func (s *stubOutboxRepo) PullPending(limit int) ([]domain.OutboxMessage, error) {
	s.pullCalls++
	if s.pullErr != nil {
		return nil, s.pullErr
	}

	if limit <= 0 || limit >= len(s.pending) {
		return append([]domain.OutboxMessage(nil), s.pending...), nil
	}
	return append([]domain.OutboxMessage(nil), s.pending[:limit]...), nil
}

func (s *stubOutboxRepo) Stats() (domain.OutboxStats, error) {
	s.statsCalls++
	if s.statsErr != nil {
		return domain.OutboxStats{}, s.statsErr
	}
	if s.stats != (domain.OutboxStats{}) {
		return s.stats, nil
	}

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
	return s.markSentErr
}

func (s *stubOutboxRepo) MarkFailed(id string) error {
	s.failedIDs = append(s.failedIDs, id)
	return s.markFailedErr
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
