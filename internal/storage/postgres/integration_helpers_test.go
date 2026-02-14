package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const defaultLocalIntegrationDSN = "postgres://oms:oms@localhost:5432/oms?sslmode=disable"

func openPostgresStoreForIntegrationTest(t *testing.T) *Store {
	t.Helper()

	store := openRawPostgresStoreForIntegrationTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := store.MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	truncateAllTablesForIntegrationTest(t, store)

	return store
}

func openRawPostgresStoreForIntegrationTest(t *testing.T) *Store {
	t.Helper()

	candidates := []string{
		strings.TrimSpace(os.Getenv("OMS_POSTGRES_TEST_DSN")),
		strings.TrimSpace(os.Getenv("OMS_POSTGRES_DSN")),
		defaultLocalIntegrationDSN,
	}

	seen := map[string]struct{}{}
	var openErrs []string
	for _, dsn := range candidates {
		dsn = strings.TrimSpace(dsn)
		if dsn == "" {
			continue
		}
		if _, ok := seen[dsn]; ok {
			continue
		}
		seen[dsn] = struct{}{}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		store, err := Open(ctx, dsn)
		cancel()
		if err == nil {
			t.Cleanup(func() {
				_ = store.Close()
			})
			return store
		}
		openErrs = append(openErrs, fmt.Sprintf("%s: %v", dsn, err))
	}

	t.Skipf("postgres is not available for integration tests: %s", strings.Join(openErrs, " | "))
	return nil
}

func truncateAllTablesForIntegrationTest(t *testing.T, store *Store) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := store.DB().ExecContext(ctx, `
		TRUNCATE TABLE
			idempotency_keys,
			outbox_messages,
			timeline_events,
			order_items,
			orders
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate integration tables: %v", err)
	}
}
