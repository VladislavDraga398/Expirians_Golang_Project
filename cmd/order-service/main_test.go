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
		envDynamicPricingEnabled:       "true",
		envDynamicPricingBaseFeeMinor:  "199",
		envDynamicPricingWeather:       "0.7",
		envDynamicPricingTraffic:       "0.4",
		envDynamicPricingCourierLoad:   "0.85",
		envDynamicPricingWeatherMaxBps: "1200",
		envDynamicPricingTrafficMaxBps: "1800",
		envDynamicPricingLoadMaxBps:    "900",
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
	if !cfg.DynamicPricingEnabled {
		t.Fatal("expected DynamicPricingEnabled=true")
	}
	if cfg.DynamicPricingBaseFeeMinor != 199 {
		t.Fatalf("unexpected dynamic pricing base fee: %d", cfg.DynamicPricingBaseFeeMinor)
	}
	if cfg.DynamicPricingDefaultWeatherSeverity != 0.7 {
		t.Fatalf("unexpected dynamic pricing weather severity: %f", cfg.DynamicPricingDefaultWeatherSeverity)
	}
	if cfg.DynamicPricingDefaultTrafficSeverity != 0.4 {
		t.Fatalf("unexpected dynamic pricing traffic severity: %f", cfg.DynamicPricingDefaultTrafficSeverity)
	}
	if cfg.DynamicPricingDefaultCourierLoad != 0.85 {
		t.Fatalf("unexpected dynamic pricing courier load: %f", cfg.DynamicPricingDefaultCourierLoad)
	}
	if cfg.DynamicPricingWeatherMaxBps != 1200 {
		t.Fatalf("unexpected dynamic pricing weather bps: %d", cfg.DynamicPricingWeatherMaxBps)
	}
	if cfg.DynamicPricingTrafficMaxBps != 1800 {
		t.Fatalf("unexpected dynamic pricing traffic bps: %d", cfg.DynamicPricingTrafficMaxBps)
	}
	if cfg.DynamicPricingLoadMaxBps != 900 {
		t.Fatalf("unexpected dynamic pricing load bps: %d", cfg.DynamicPricingLoadMaxBps)
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
		envDynamicPricingEnabled:       "not-bool",
		envDynamicPricingBaseFeeMinor:  "-1",
		envDynamicPricingWeather:       "1.5",
		envDynamicPricingTraffic:       "-0.1",
		envDynamicPricingCourierLoad:   "oops",
		envDynamicPricingWeatherMaxBps: "-10",
		envDynamicPricingTrafficMaxBps: "-20",
		envDynamicPricingLoadMaxBps:    "-30",
	}))

	if len(warnings) != 17 {
		t.Fatalf("expected 17 warnings, got %d", len(warnings))
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
	if cfg.DynamicPricingEnabled != defaultCfg.DynamicPricingEnabled {
		t.Fatal("expected DynamicPricingEnabled to keep default on invalid value")
	}
	if cfg.DynamicPricingBaseFeeMinor != defaultCfg.DynamicPricingBaseFeeMinor {
		t.Fatal("expected DynamicPricingBaseFeeMinor to keep default on invalid value")
	}
	if cfg.DynamicPricingDefaultWeatherSeverity != defaultCfg.DynamicPricingDefaultWeatherSeverity {
		t.Fatal("expected DynamicPricingDefaultWeatherSeverity to keep default on invalid value")
	}
	if cfg.DynamicPricingDefaultTrafficSeverity != defaultCfg.DynamicPricingDefaultTrafficSeverity {
		t.Fatal("expected DynamicPricingDefaultTrafficSeverity to keep default on invalid value")
	}
	if cfg.DynamicPricingDefaultCourierLoad != defaultCfg.DynamicPricingDefaultCourierLoad {
		t.Fatal("expected DynamicPricingDefaultCourierLoad to keep default on invalid value")
	}
	if cfg.DynamicPricingWeatherMaxBps != defaultCfg.DynamicPricingWeatherMaxBps {
		t.Fatal("expected DynamicPricingWeatherMaxBps to keep default on invalid value")
	}
	if cfg.DynamicPricingTrafficMaxBps != defaultCfg.DynamicPricingTrafficMaxBps {
		t.Fatal("expected DynamicPricingTrafficMaxBps to keep default on invalid value")
	}
	if cfg.DynamicPricingLoadMaxBps != defaultCfg.DynamicPricingLoadMaxBps {
		t.Fatal("expected DynamicPricingLoadMaxBps to keep default on invalid value")
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

func TestParseFloat(t *testing.T) {
	value, err := parseFloat(" 0.75 ", func(v float64) bool { return v >= 0 && v <= 1 }, "must be within [0,1]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != 0.75 {
		t.Fatalf("unexpected value: %f", value)
	}

	if _, err := parseFloat("2", func(v float64) bool { return v >= 0 && v <= 1 }, "must be within [0,1]"); err == nil {
		t.Fatal("expected validation error")
	}
}

func mapLookup(values map[string]string) envLookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
