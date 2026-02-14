package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	healthcheck "github.com/vladislavdragonenkov/oms/internal/health"
	"github.com/vladislavdragonenkov/oms/internal/messaging/kafka"
	grpcsvc "github.com/vladislavdragonenkov/oms/internal/service/grpc"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

func TestRun_MemoryGracefulShutdown(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")

	cfg := DefaultConfig()
	cfg.GRPCAddr = "127.0.0.1:0"
	cfg.MetricsAddr = "127.0.0.1:0"
	cfg.StorageDriver = StorageDriverMemory

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	err := Run(ctx, cfg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRun_InvalidStorageDriver(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StorageDriver = "invalid-driver"
	cfg.GRPCAddr = "127.0.0.1:0"
	cfg.MetricsAddr = "127.0.0.1:0"

	err := Run(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("expected unsupported storage driver error, got %v", err)
	}
}

func TestInitRuntimeDependencies_PostgresSuccess(t *testing.T) {
	dsn := postgresTestDSNCandidate()
	if dsn == "" {
		t.Skip("postgres dsn is not available")
	}

	cfg := DefaultConfig()
	cfg.StorageDriver = StorageDriverPostgres
	cfg.PostgresDSN = dsn
	cfg.PostgresAutoMigrate = true

	deps, err := initRuntimeDependencies(context.Background(), cfg, log.WithField("test", "postgres-init"))
	if err != nil {
		t.Skipf("postgres is not available for app integration test: %v", err)
	}
	if deps.closeFn != nil {
		defer func() { _ = deps.closeFn() }()
	}

	if deps.repo == nil || deps.outboxRepo == nil || deps.timelineRepo == nil || deps.idempotencyRepo == nil {
		t.Fatalf("postgres dependencies must be initialized: %+v", deps)
	}
	if deps.storageChecker == nil {
		t.Fatal("expected non-nil storage checker for postgres")
	}
	check := deps.storageChecker.Check()
	if check.Status != healthcheck.StatusHealthy {
		t.Fatalf("expected healthy storage checker, got %+v", check)
	}
}

func TestShutdownHelpers(t *testing.T) {
	logger := log.WithField("test", "shutdown")

	orderService := grpcsvc.NewOrderService(
		memory.NewOrderRepository(),
		memory.NewTimelineRepository(),
		memory.NewIdempotencyRepository(),
		saga.NewNoop(nil),
		logger,
	)
	shutdownOrderService(orderService, logger)
	shutdownOrderService(nil, logger)

	cancelCalled := false
	done := make(chan struct{})
	close(done)
	shutdownOutboxWorker(func() { cancelCalled = true }, done, logger)
	if !cancelCalled {
		t.Fatal("expected outbox cancel func to be called")
	}

	shutdownOutboxWorker(nil, nil, logger)

	closeKafkaProducer(nil, logger)
}

func TestCloseKafkaProducer_NonNil(t *testing.T) {
	producer, err := kafka.NewProducer([]string{"localhost:9092"})
	if err != nil {
		t.Skipf("kafka is not available for integration test: %v", err)
	}
	closeKafkaProducer(producer, log.WithField("test", "kafka-close"))
}

func postgresTestDSNCandidate() string {
	return strings.TrimSpace(os.Getenv("OMS_POSTGRES_TEST_DSN"))
}
