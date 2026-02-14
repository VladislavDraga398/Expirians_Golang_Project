package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestIdempotencyRepository_PostgresCreateGetAndMarkDone(t *testing.T) {
	store := openPostgresStoreForIdempotencyTest(t)
	repo := NewIdempotencyRepository(store)

	key := "idem-test-key-done"
	hash := "req-hash-1"
	ttl := time.Now().UTC().Add(2 * time.Hour).Round(time.Second)

	created, err := repo.CreateProcessing(key, hash, ttl)
	require.NoError(t, err)
	require.Equal(t, domain.IdempotencyStatusProcessing, created.Status)

	err = repo.MarkDone(key, []byte(`{"result":"ok"}`), 200)
	require.NoError(t, err)

	got, err := repo.Get(key)
	require.NoError(t, err)
	require.Equal(t, hash, got.RequestHash)
	require.Equal(t, domain.IdempotencyStatusDone, got.Status)
	require.Equal(t, 200, got.HTTPStatus)
	require.JSONEq(t, `{"result":"ok"}`, string(got.ResponseBody))
	require.True(t, got.TTLAt.Equal(ttl), "ttl mismatch: expected %s, got %s", ttl, got.TTLAt)
}

func TestIdempotencyRepository_PostgresConflictAndHashMismatch(t *testing.T) {
	store := openPostgresStoreForIdempotencyTest(t)
	repo := NewIdempotencyRepository(store)

	ttl := time.Now().UTC().Add(time.Hour)
	_, err := repo.CreateProcessing("idem-test-key-conflict", "req-hash-a", ttl)
	require.NoError(t, err)

	_, err = repo.CreateProcessing("idem-test-key-conflict", "req-hash-a", ttl)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrIdempotencyKeyAlreadyExists))

	_, err = repo.CreateProcessing("idem-test-key-conflict", "req-hash-b", ttl)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrIdempotencyHashMismatch))
}

func TestIdempotencyRepository_PostgresDeleteExpired(t *testing.T) {
	store := openPostgresStoreForIdempotencyTest(t)
	repo := NewIdempotencyRepository(store)

	now := time.Now().UTC()
	_, err := repo.CreateProcessing("idem-expired-1", "h1", now.Add(-5*time.Minute))
	require.NoError(t, err)
	_, err = repo.CreateProcessing("idem-expired-2", "h2", now.Add(-4*time.Minute))
	require.NoError(t, err)
	_, err = repo.CreateProcessing("idem-expired-3", "h3", now.Add(-3*time.Minute))
	require.NoError(t, err)
	_, err = repo.CreateProcessing("idem-active-1", "h4", now.Add(time.Hour))
	require.NoError(t, err)

	removed, err := repo.DeleteExpired(now, 2)
	require.NoError(t, err)
	require.Equal(t, 2, removed)

	removed, err = repo.DeleteExpired(now, 10)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	_, err = repo.Get("idem-active-1")
	require.NoError(t, err)
}

func openPostgresStoreForIdempotencyTest(t *testing.T) *Store {
	t.Helper()

	store := openPostgresStoreForIntegrationTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := store.DB().ExecContext(ctx, `TRUNCATE TABLE idempotency_keys`)
	require.NoError(t, err)

	return store
}
