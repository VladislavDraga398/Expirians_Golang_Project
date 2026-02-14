package memory_test

import (
	"errors"
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

func TestIdempotencyRepository_CreateAndGet(t *testing.T) {
	repo := memory.NewIdempotencyRepository()
	ttl := time.Now().UTC().Add(2 * time.Hour).Round(time.Second)

	created, err := repo.CreateProcessing("idem-key-1", "hash-1", ttl)
	if err != nil {
		t.Fatalf("CreateProcessing failed: %v", err)
	}
	if created.Status != domain.IdempotencyStatusProcessing {
		t.Fatalf("expected status %s, got %s", domain.IdempotencyStatusProcessing, created.Status)
	}

	got, err := repo.Get("idem-key-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.RequestHash != "hash-1" {
		t.Fatalf("expected request_hash hash-1, got %s", got.RequestHash)
	}
	if !got.TTLAt.Equal(ttl) {
		t.Fatalf("expected ttl %s, got %s", ttl, got.TTLAt)
	}
}

func TestIdempotencyRepository_ConflictAndHashMismatch(t *testing.T) {
	repo := memory.NewIdempotencyRepository()
	ttl := time.Now().UTC().Add(time.Hour)

	if _, err := repo.CreateProcessing("idem-key-2", "hash-a", ttl); err != nil {
		t.Fatalf("CreateProcessing failed: %v", err)
	}

	if _, err := repo.CreateProcessing("idem-key-2", "hash-a", ttl); !errors.Is(err, domain.ErrIdempotencyKeyAlreadyExists) {
		t.Fatalf("expected ErrIdempotencyKeyAlreadyExists, got %v", err)
	}

	if _, err := repo.CreateProcessing("idem-key-2", "hash-b", ttl); !errors.Is(err, domain.ErrIdempotencyHashMismatch) {
		t.Fatalf("expected ErrIdempotencyHashMismatch, got %v", err)
	}
}

func TestIdempotencyRepository_MarkDoneAndDeleteExpired(t *testing.T) {
	repo := memory.NewIdempotencyRepository()

	expiredTTL := time.Now().UTC().Add(-time.Minute)
	activeTTL := time.Now().UTC().Add(time.Hour)

	if _, err := repo.CreateProcessing("idem-expired", "hash-expired", expiredTTL); err != nil {
		t.Fatalf("CreateProcessing expired failed: %v", err)
	}
	if _, err := repo.CreateProcessing("idem-active", "hash-active", activeTTL); err != nil {
		t.Fatalf("CreateProcessing active failed: %v", err)
	}

	if err := repo.MarkDone("idem-active", []byte(`{"ok":true}`), 200); err != nil {
		t.Fatalf("MarkDone failed: %v", err)
	}

	active, err := repo.Get("idem-active")
	if err != nil {
		t.Fatalf("Get active failed: %v", err)
	}
	if active.Status != domain.IdempotencyStatusDone {
		t.Fatalf("expected status %s, got %s", domain.IdempotencyStatusDone, active.Status)
	}
	if active.HTTPStatus != 200 {
		t.Fatalf("expected http status 200, got %d", active.HTTPStatus)
	}

	removed, err := repo.DeleteExpired(time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected removed=1, got %d", removed)
	}

	if _, err := repo.Get("idem-expired"); !errors.Is(err, domain.ErrIdempotencyKeyNotFound) {
		t.Fatalf("expected expired key to be deleted, got %v", err)
	}
}
