package postgres

import (
	"context"
	"testing"
	"time"
)

func TestMigrator_PostgresLifecycle(t *testing.T) {
	store := openRawPostgresStoreForIntegrationTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Reset migration state first.
	if err := store.MigrateDown(ctx, 100); err != nil {
		t.Fatalf("migrate down reset: %v", err)
	}

	version, count, err := store.MigrationStatus(ctx)
	if err != nil {
		t.Fatalf("migration status after reset: %v", err)
	}
	if version != 0 || count != 0 {
		t.Fatalf("unexpected status after reset: version=%d count=%d", version, count)
	}

	if err := store.MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up all: %v", err)
	}
	version, count, err = store.MigrationStatus(ctx)
	if err != nil {
		t.Fatalf("migration status after up all: %v", err)
	}
	if version != 2 || count != 2 {
		t.Fatalf("unexpected status after up all: version=%d count=%d", version, count)
	}

	// Idempotent up should keep state unchanged.
	if err := store.MigrateUp(ctx, 0); err != nil {
		t.Fatalf("idempotent migrate up: %v", err)
	}
	version, count, err = store.MigrationStatus(ctx)
	if err != nil {
		t.Fatalf("migration status after idempotent up: %v", err)
	}
	if version != 2 || count != 2 {
		t.Fatalf("unexpected status after idempotent up: version=%d count=%d", version, count)
	}

	if err := store.MigrateDown(ctx, 1); err != nil {
		t.Fatalf("migrate down 1: %v", err)
	}
	version, count, err = store.MigrationStatus(ctx)
	if err != nil {
		t.Fatalf("migration status after down 1: %v", err)
	}
	if version != 1 || count != 1 {
		t.Fatalf("unexpected status after down 1: version=%d count=%d", version, count)
	}

	if err := store.MigrateDown(ctx, 0); err != nil {
		t.Fatalf("migrate down default step: %v", err)
	}
	version, count, err = store.MigrationStatus(ctx)
	if err != nil {
		t.Fatalf("migration status after down default: %v", err)
	}
	if version != 0 || count != 0 {
		t.Fatalf("unexpected status after down default: version=%d count=%d", version, count)
	}

	// No-op down on empty state.
	if err := store.MigrateDown(ctx, 1); err != nil {
		t.Fatalf("migrate down on empty should be no-op: %v", err)
	}
}

func TestMigrator_GuardsAndUnsupportedDirection(t *testing.T) {
	var nilStore *Store
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := nilStore.MigrateUp(ctx, 0); err == nil {
		t.Fatal("expected error for nil store MigrateUp")
	}
	if err := nilStore.MigrateDown(ctx, 1); err == nil {
		t.Fatal("expected error for nil store MigrateDown")
	}
	if _, _, err := nilStore.MigrationStatus(ctx); err == nil {
		t.Fatal("expected error for nil store MigrationStatus")
	}

	store := openRawPostgresStoreForIntegrationTest(t)
	if err := store.migrate(ctx, migrationDirection("invalid"), 0); err == nil {
		t.Fatal("expected unsupported direction error")
	}
}
