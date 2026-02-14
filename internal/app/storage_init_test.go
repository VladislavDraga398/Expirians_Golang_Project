package app

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestInitRuntimeDependencies_Memory(t *testing.T) {
	t.Parallel()

	deps, err := initRuntimeDependencies(context.Background(), Config{
		StorageDriver: StorageDriverMemory,
	}, log.WithField("test", "memory-storage"))
	if err != nil {
		t.Fatalf("initRuntimeDependencies(memory) failed: %v", err)
	}
	if deps.repo == nil {
		t.Fatal("repo should not be nil for memory storage")
	}
	if deps.outboxRepo == nil {
		t.Fatal("outboxRepo should not be nil for memory storage")
	}
	if deps.timelineRepo == nil {
		t.Fatal("timelineRepo should not be nil for memory storage")
	}
	if deps.idempotencyRepo == nil {
		t.Fatal("idempotencyRepo should not be nil for memory storage")
	}
}

func TestInitRuntimeDependencies_PostgresRequiresDSN(t *testing.T) {
	t.Parallel()

	_, err := initRuntimeDependencies(context.Background(), Config{
		StorageDriver: StorageDriverPostgres,
	}, log.WithField("test", "postgres-missing-dsn"))
	if err == nil {
		t.Fatal("expected error when postgres driver is selected without DSN")
	}
}

func TestInitRuntimeDependencies_UnsupportedDriver(t *testing.T) {
	t.Parallel()

	_, err := initRuntimeDependencies(context.Background(), Config{
		StorageDriver: "sqlite",
	}, log.WithField("test", "unsupported-driver"))
	if err == nil {
		t.Fatal("expected error for unsupported storage driver")
	}
}
