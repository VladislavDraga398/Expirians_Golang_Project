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

	migrations, err := loadMigrationsFromFS(migrationsFS)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) < 2 {
		t.Fatalf("expected at least 2 migrations, got %d", len(migrations))
	}

	assertStatus := func(stage string, wantVersion int64, wantCount int) {
		t.Helper()

		version, count, statusErr := store.MigrationStatus(ctx)
		if statusErr != nil {
			t.Fatalf("migration status %s: %v", stage, statusErr)
		}
		if version != wantVersion || count != wantCount {
			t.Fatalf(
				"unexpected status %s: version=%d count=%d, want version=%d count=%d",
				stage,
				version,
				count,
				wantVersion,
				wantCount,
			)
		}
	}

	// Reset migration state first.
	if err := store.MigrateDown(ctx, 100); err != nil {
		t.Fatalf("migrate down reset: %v", err)
	}
	assertStatus("after reset", 0, 0)

	if err := store.MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up all: %v", err)
	}
	wantCount := len(migrations)
	wantVersion := migrations[wantCount-1].Version
	assertStatus("after up all", wantVersion, wantCount)

	// Idempotent up should keep state unchanged.
	if err := store.MigrateUp(ctx, 0); err != nil {
		t.Fatalf("idempotent migrate up: %v", err)
	}
	assertStatus("after idempotent up", wantVersion, wantCount)

	if err := store.MigrateDown(ctx, 1); err != nil {
		t.Fatalf("migrate down 1: %v", err)
	}
	wantCount--
	wantVersion = migrations[wantCount-1].Version
	assertStatus("after down 1", wantVersion, wantCount)

	if err := store.MigrateDown(ctx, 0); err != nil {
		t.Fatalf("migrate down default step: %v", err)
	}
	wantCount--
	wantVersion = migrations[wantCount-1].Version
	assertStatus("after down default", wantVersion, wantCount)

	for wantCount > 0 {
		if err := store.MigrateDown(ctx, 1); err != nil {
			t.Fatalf("migrate down to zero: %v", err)
		}
		wantCount--
		if wantCount == 0 {
			wantVersion = 0
		} else {
			wantVersion = migrations[wantCount-1].Version
		}
		assertStatus("drain down", wantVersion, wantCount)
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
