package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/storage/postgres"
)

const (
	defaultTimeout = 30 * time.Second
)

func main() {
	var (
		direction string
		steps     int
		dsn       string
	)

	flag.StringVar(&direction, "direction", "up", "migration direction: up|down|status")
	flag.IntVar(&steps, "steps", 0, "number of migrations to apply/rollback (0=all for up, 1 for down)")
	flag.StringVar(&dsn, "dsn", "", "PostgreSQL DSN (fallback: OMS_POSTGRES_DSN)")
	flag.Parse()

	if strings.TrimSpace(dsn) == "" {
		dsn = strings.TrimSpace(os.Getenv("OMS_POSTGRES_DSN"))
	}
	if dsn == "" {
		fail("OMS_POSTGRES_DSN (or -dsn) is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		fail("open postgres store: %v", err)
	}
	defer store.Close()

	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "up":
		if err := store.MigrateUp(ctx, steps); err != nil {
			fail("migrate up failed: %v", err)
		}
		version, count, err := store.MigrationStatus(ctx)
		if err != nil {
			fail("migration status failed: %v", err)
		}
		fmt.Printf("migrate up ok: version=%d applied=%d\n", version, count)
	case "down":
		if steps <= 0 {
			steps = 1
		}
		if err := store.MigrateDown(ctx, steps); err != nil {
			fail("migrate down failed: %v", err)
		}
		version, count, err := store.MigrationStatus(ctx)
		if err != nil {
			fail("migration status failed: %v", err)
		}
		fmt.Printf("migrate down ok: version=%d applied=%d\n", version, count)
	case "status":
		version, count, err := store.MigrationStatus(ctx)
		if err != nil {
			fail("migration status failed: %v", err)
		}
		fmt.Printf("migration status: version=%d applied=%d\n", version, count)
	default:
		fail("unsupported direction: %s (use up|down|status)", direction)
	}
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
