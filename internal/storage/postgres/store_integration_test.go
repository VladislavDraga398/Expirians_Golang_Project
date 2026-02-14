package postgres

import (
	"context"
	"testing"
	"time"
)

func TestStore_PostgresOpenPingEnsureAndClose(t *testing.T) {
	store := openRawPostgresStoreForIntegrationTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping store: %v", err)
	}
	if store.DB() == nil {
		t.Fatal("expected non-nil raw DB")
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
}

func TestStore_NilGuards(t *testing.T) {
	var store *Store

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := store.Ping(ctx); err == nil {
		t.Fatal("expected ping error for nil store")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close nil store should not fail: %v", err)
	}
}

func TestStore_OpenInvalidDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, err := Open(ctx, "postgres://invalid:invalid@127.0.0.1:1/invalid?sslmode=disable")
	if err == nil {
		t.Fatal("expected open error for invalid dsn")
	}
}
