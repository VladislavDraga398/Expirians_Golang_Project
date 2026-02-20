package idempotency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
