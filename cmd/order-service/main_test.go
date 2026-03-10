package main

import (
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/app"
)

func TestReadConfigFromEnv_Defaults(t *testing.T) {
	cfg, warnings := readConfigFromEnv(mapLookup(nil))

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %d", len(warnings))
	}

	if cfg != app.DefaultConfig() {
		t.Fatalf("expected default config, got %#v", cfg)
	}
}

func TestReadConfigFromEnv_ValidOverrides(t *testing.T) {
	cfg, warnings := readConfigFromEnv(mapLookup(map[string]string{
		envGRPCAddr:                    "localhost:50051",
		envMetricsAddr:                 "localhost:9090",
		envStorageDriver:               " PoStGrEs ",
		envPostgresDSN:                 " postgres://oms:oms@localhost:5432/oms?sslmode=disable ",
		envPostgresAutoMigrate:         "off",
		envAllowMockIntegrations:       "yes",
		envOutboxPollInterval:          "2s",
		envOutboxBatchSize:             "42",
		envOutboxMaxAttempts:           "7",
		envOutboxRetryDelay:            "0s",
		envOutboxMaxPending:            "0",
		envIdempotencyCleanupInterval:  "30m",
		envIdempotencyCleanupBatchSize: "123",
	}))

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %d", len(warnings))
	}

	if cfg.GRPCAddr != "localhost:50051" {
		t.Fatalf("unexpected grpc addr: %s", cfg.GRPCAddr)
	}
	if cfg.MetricsAddr != "localhost:9090" {
		t.Fatalf("unexpected metrics addr: %s", cfg.MetricsAddr)
	}
	if cfg.StorageDriver != "postgres" {
		t.Fatalf("unexpected storage driver: %s", cfg.StorageDriver)
	}
	if cfg.PostgresDSN != "postgres://oms:oms@localhost:5432/oms?sslmode=disable" {
		t.Fatalf("unexpected postgres dsn: %s", cfg.PostgresDSN)
	}
	if cfg.PostgresAutoMigrate {
		t.Fatal("expected PostgresAutoMigrate=false")
	}
	if !cfg.AllowMockIntegrations {
		t.Fatal("expected AllowMockIntegrations=true")
	}
	if cfg.OutboxPollInterval != 2*time.Second {
		t.Fatalf("unexpected poll interval: %s", cfg.OutboxPollInterval)
	}
	if cfg.OutboxBatchSize != 42 {
		t.Fatalf("unexpected batch size: %d", cfg.OutboxBatchSize)
	}
	if cfg.OutboxMaxAttempts != 7 {
		t.Fatalf("unexpected max attempts: %d", cfg.OutboxMaxAttempts)
	}
	if cfg.OutboxRetryDelay != 0 {
		t.Fatalf("unexpected retry delay: %s", cfg.OutboxRetryDelay)
	}
	if cfg.OutboxMaxPending != 0 {
		t.Fatalf("unexpected max pending: %d", cfg.OutboxMaxPending)
	}
	if cfg.IdempotencyCleanupInterval != 30*time.Minute {
		t.Fatalf("unexpected idempotency cleanup interval: %s", cfg.IdempotencyCleanupInterval)
	}
	if cfg.IdempotencyCleanupBatchSize != 123 {
		t.Fatalf("unexpected idempotency cleanup batch size: %d", cfg.IdempotencyCleanupBatchSize)
	}
}

func TestReadConfigFromEnv_InvalidValuesFallbackToDefaults(t *testing.T) {
	defaultCfg := app.DefaultConfig()

	cfg, warnings := readConfigFromEnv(mapLookup(map[string]string{
		envPostgresAutoMigrate:         "not-bool",
		envAllowMockIntegrations:       "not-bool",
		envOutboxPollInterval:          "-1s",
		envOutboxBatchSize:             "0",
		envOutboxMaxAttempts:           "bad",
		envOutboxRetryDelay:            "invalid",
		envOutboxMaxPending:            "-2",
		envIdempotencyCleanupInterval:  "invalid",
		envIdempotencyCleanupBatchSize: "0",
	}))

	if len(warnings) != 9 {
		t.Fatalf("expected 9 warnings, got %d", len(warnings))
	}

	if cfg.PostgresAutoMigrate != defaultCfg.PostgresAutoMigrate {
		t.Fatal("expected PostgresAutoMigrate to keep default on invalid value")
	}
	if cfg.AllowMockIntegrations != defaultCfg.AllowMockIntegrations {
		t.Fatal("expected AllowMockIntegrations to keep default on invalid value")
	}
	if cfg.OutboxPollInterval != defaultCfg.OutboxPollInterval {
		t.Fatal("expected OutboxPollInterval to keep default on invalid value")
	}
	if cfg.OutboxBatchSize != defaultCfg.OutboxBatchSize {
		t.Fatal("expected OutboxBatchSize to keep default on invalid value")
	}
	if cfg.OutboxMaxAttempts != defaultCfg.OutboxMaxAttempts {
		t.Fatal("expected OutboxMaxAttempts to keep default on invalid value")
	}
	if cfg.OutboxRetryDelay != defaultCfg.OutboxRetryDelay {
		t.Fatal("expected OutboxRetryDelay to keep default on invalid value")
	}
	if cfg.OutboxMaxPending != defaultCfg.OutboxMaxPending {
		t.Fatal("expected OutboxMaxPending to keep default on invalid value")
	}
	if cfg.IdempotencyCleanupInterval != defaultCfg.IdempotencyCleanupInterval {
		t.Fatal("expected IdempotencyCleanupInterval to keep default on invalid value")
	}
	if cfg.IdempotencyCleanupBatchSize != defaultCfg.IdempotencyCleanupBatchSize {
		t.Fatal("expected IdempotencyCleanupBatchSize to keep default on invalid value")
	}
}

func TestParseBool(t *testing.T) {
	trueValue, err := parseBool(" YES ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !trueValue {
		t.Fatal("expected true result")
	}

	falseValue, err := parseBool("off")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if falseValue {
		t.Fatal("expected false result")
	}

	if _, err := parseBool("sometimes"); err == nil {
		t.Fatal("expected error for invalid bool value")
	}
}

func TestParseInt(t *testing.T) {
	value, err := parseInt(" 12 ", func(v int) bool { return v > 0 }, "must be > 0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != 12 {
		t.Fatalf("unexpected value: %d", value)
	}

	if _, err := parseInt("0", func(v int) bool { return v > 0 }, "must be > 0"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseDuration(t *testing.T) {
	value, err := parseDuration(" 250ms ", func(v time.Duration) bool { return v >= 0 }, "must be >= 0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != 250*time.Millisecond {
		t.Fatalf("unexpected value: %s", value)
	}

	if _, err := parseDuration("-1ms", func(v time.Duration) bool { return v >= 0 }, "must be >= 0"); err == nil {
		t.Fatal("expected validation error")
	}
}

func mapLookup(values map[string]string) envLookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
