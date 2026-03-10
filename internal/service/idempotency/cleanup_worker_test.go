package idempotency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

var _ domain.IdempotencyRepository = (*stubCleanupRepo)(nil)

func TestCleanupWorker_DeleteExpired_Batches(t *testing.T) {
	t.Parallel()

	repo := &stubCleanupRepo{
		deleteResults: []int{2, 2, 1},
	}

	worker := NewCleanupWorker(repo, WithBatchSize(2))

	deleted, err := worker.DeleteExpired(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}

	if deleted != 5 {
		t.Fatalf("unexpected deleted total: got=%d want=5", deleted)
	}

	if calls := repo.calls(); calls != 3 {
		t.Fatalf("unexpected delete calls: got=%d want=3", calls)
	}
}

func TestCleanupWorker_DeleteExpired_Error(t *testing.T) {
	t.Parallel()

	repo := &stubCleanupRepo{
		deleteErrors: []error{errors.New("boom")},
	}

	worker := NewCleanupWorker(repo, WithBatchSize(10))

	deleted, err := worker.DeleteExpired(context.Background(), time.Now().UTC())
	if err == nil {
		t.Fatal("expected DeleteExpired error")
	}
	if deleted != 0 {
		t.Fatalf("unexpected deleted total: got=%d want=0", deleted)
	}
}

func TestCleanupWorker_Run_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	repo := &stubCleanupRepo{
		deleteResults: []int{0, 0, 0},
	}

	worker := NewCleanupWorker(
		repo,
		WithInterval(5*time.Millisecond),
		WithBatchSize(10),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop on context cancel")
	}

	if calls := repo.calls(); calls == 0 {
		t.Fatal("expected cleanup to be called at least once")
	}
}

func TestNewCleanupWorker_OptionsAndNormalization(t *testing.T) {
	t.Parallel()

	logger := log.WithField("test", "cleanup-worker")
	worker := NewCleanupWorker(
		&stubCleanupRepo{},
		WithLogger(logger),
		WithInterval(0),
		WithBatchSize(0),
	)

	if worker.logger != logger {
		t.Fatal("expected custom logger to be used")
	}
	if worker.interval != defaultCleanupInterval {
		t.Fatalf("expected default interval %s, got %s", defaultCleanupInterval, worker.interval)
	}
	if worker.batchSize != defaultCleanupBatchSize {
		t.Fatalf("expected default batch size %d, got %d", defaultCleanupBatchSize, worker.batchSize)
	}
}

func TestCleanupWorker_Run_DisabledWhenRepoNil(t *testing.T) {
	t.Parallel()

	worker := NewCleanupWorker(nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("worker.Run should return immediately when repo is nil")
	}
}

func TestCleanupWorker_Cleanup_ErrorAndCanceled(t *testing.T) {
	t.Parallel()

	errRepo := &stubCleanupRepo{
		deleteErrors: []error{errors.New("cleanup error")},
	}
	errWorker := NewCleanupWorker(errRepo, WithBatchSize(10))
	errWorker.cleanup(context.Background(), time.Now().UTC())
	if calls := errRepo.calls(); calls != 1 {
		t.Fatalf("expected 1 delete call on error path, got %d", calls)
	}

	cancelRepo := &stubCleanupRepo{
		deleteErrors: []error{context.Canceled},
	}
	cancelWorker := NewCleanupWorker(cancelRepo, WithBatchSize(10))
	cancelWorker.cleanup(context.Background(), time.Now().UTC())
	if calls := cancelRepo.calls(); calls != 1 {
		t.Fatalf("expected 1 delete call on canceled path, got %d", calls)
	}
}

func TestCleanupWorker_DeleteExpired_ContextCanceled(t *testing.T) {
	t.Parallel()

	repo := &stubCleanupRepo{
		deleteResults: []int{1},
	}
	worker := NewCleanupWorker(repo, WithBatchSize(10))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	deleted, err := worker.DeleteExpired(ctx, time.Now().UTC())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted records, got %d", deleted)
	}
	if calls := repo.calls(); calls != 0 {
		t.Fatalf("expected no delete calls when ctx already canceled, got %d", calls)
	}
}

type stubCleanupRepo struct {
	mu sync.Mutex

	deleteResults []int
	deleteErrors  []error
	callCount     int
}

func (s *stubCleanupRepo) CreateProcessing(string, string, time.Time) (domain.IdempotencyRecord, error) {
	panic("not implemented")
}

func (s *stubCleanupRepo) Get(string) (domain.IdempotencyRecord, error) {
	panic("not implemented")
}

func (s *stubCleanupRepo) MarkDone(string, []byte, int) error {
	panic("not implemented")
}

func (s *stubCleanupRepo) MarkFailed(string, []byte, int) error {
	panic("not implemented")
}

func (s *stubCleanupRepo) DeleteExpired(_ time.Time, _ int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callCount++

	if len(s.deleteErrors) > 0 {
		err := s.deleteErrors[0]
		s.deleteErrors = s.deleteErrors[1:]
		if err != nil {
			return 0, err
		}
	}

	if len(s.deleteResults) == 0 {
		return 0, nil
	}
	result := s.deleteResults[0]
	s.deleteResults = s.deleteResults[1:]
	return result, nil
}

func (s *stubCleanupRepo) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}
