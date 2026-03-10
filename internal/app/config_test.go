package app

import (
	"testing"
	"time"
)

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.GRPCAddr != ":50051" {
		t.Errorf("expected GRPCAddr :50051, got %s", cfg.GRPCAddr)
	}

	if cfg.MetricsAddr != ":9090" {
		t.Errorf("expected MetricsAddr :9090, got %s", cfg.MetricsAddr)
	}

	if cfg.StorageDriver != StorageDriverMemory {
		t.Errorf("expected StorageDriver %s, got %s", StorageDriverMemory, cfg.StorageDriver)
	}

	if !cfg.PostgresAutoMigrate {
		t.Error("expected PostgresAutoMigrate to be true")
	}
	if cfg.OutboxPollInterval <= 0 {
		t.Error("expected OutboxPollInterval to be > 0")
	}
	if cfg.OutboxBatchSize <= 0 {
		t.Error("expected OutboxBatchSize to be > 0")
	}
	if cfg.OutboxMaxAttempts <= 0 {
		t.Error("expected OutboxMaxAttempts to be > 0")
	}
	if cfg.OutboxRetryDelay < 0 {
		t.Error("expected OutboxRetryDelay to be >= 0")
	}
	if cfg.OutboxMaxPending <= 0 {
		t.Error("expected OutboxMaxPending to be > 0")
	}
	if cfg.IdempotencyCleanupInterval <= 0 {
		t.Error("expected IdempotencyCleanupInterval to be > 0")
	}
	if cfg.IdempotencyCleanupBatchSize <= 0 {
		t.Error("expected IdempotencyCleanupBatchSize to be > 0")
	}
}

func TestConfig_CustomValues(t *testing.T) {
	cfg := Config{
		GRPCAddr:                    ":8080",
		MetricsAddr:                 ":9091",
		StorageDriver:               StorageDriverPostgres,
		PostgresDSN:                 "postgres://oms:oms@localhost:5432/oms?sslmode=disable",
		PostgresAutoMigrate:         false,
		OutboxPollInterval:          2 * time.Second,
		OutboxBatchSize:             50,
		OutboxMaxAttempts:           5,
		OutboxRetryDelay:            time.Second,
		OutboxMaxPending:            200,
		IdempotencyCleanupInterval:  5 * time.Minute,
		IdempotencyCleanupBatchSize: 300,
	}

	if cfg.GRPCAddr != ":8080" {
		t.Errorf("expected GRPCAddr :8080, got %s", cfg.GRPCAddr)
	}

	if cfg.MetricsAddr != ":9091" {
		t.Errorf("expected MetricsAddr :9091, got %s", cfg.MetricsAddr)
	}

	if cfg.StorageDriver != StorageDriverPostgres {
		t.Errorf("expected StorageDriver %s, got %s", StorageDriverPostgres, cfg.StorageDriver)
	}

	if cfg.PostgresDSN == "" {
		t.Error("expected PostgresDSN to be set")
	}

	if cfg.PostgresAutoMigrate {
		t.Error("expected PostgresAutoMigrate to be false")
	}
	if cfg.IdempotencyCleanupInterval != 5*time.Minute {
		t.Errorf("expected IdempotencyCleanupInterval 5m, got %s", cfg.IdempotencyCleanupInterval)
	}
	if cfg.IdempotencyCleanupBatchSize != 300 {
		t.Errorf("expected IdempotencyCleanupBatchSize 300, got %d", cfg.IdempotencyCleanupBatchSize)
	}
}

func TestConfig_EmptyValues(t *testing.T) {
	cfg := Config{}

	if cfg.GRPCAddr != "" {
		t.Errorf("expected empty GRPCAddr, got %s", cfg.GRPCAddr)
	}

	if cfg.MetricsAddr != "" {
		t.Errorf("expected empty MetricsAddr, got %s", cfg.MetricsAddr)
	}

	if cfg.StorageDriver != "" {
		t.Errorf("expected empty StorageDriver, got %s", cfg.StorageDriver)
	}

	if cfg.PostgresDSN != "" {
		t.Errorf("expected empty PostgresDSN, got %s", cfg.PostgresDSN)
	}

	if cfg.PostgresAutoMigrate {
		t.Error("expected PostgresAutoMigrate to be false for zero value")
	}
}

func TestConfig_PortFormats(t *testing.T) {
	testCases := []struct {
		name        string
		grpcAddr    string
		metricsAddr string
	}{
		{
			name:        "standard ports",
			grpcAddr:    ":50051",
			metricsAddr: ":9090",
		},
		{
			name:        "custom ports",
			grpcAddr:    ":8080",
			metricsAddr: ":8081",
		},
		{
			name:        "with host",
			grpcAddr:    "localhost:50051",
			metricsAddr: "localhost:9090",
		},
		{
			name:        "with IP",
			grpcAddr:    "0.0.0.0:50051",
			metricsAddr: "0.0.0.0:9090",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				GRPCAddr:    tc.grpcAddr,
				MetricsAddr: tc.metricsAddr,
			}

			if cfg.GRPCAddr != tc.grpcAddr {
				t.Errorf("expected GRPCAddr %s, got %s", tc.grpcAddr, cfg.GRPCAddr)
			}

			if cfg.MetricsAddr != tc.metricsAddr {
				t.Errorf("expected MetricsAddr %s, got %s", tc.metricsAddr, cfg.MetricsAddr)
			}
		})
	}
}

func TestDefaultConfig_NotNil(t *testing.T) {
	cfg := DefaultConfig()

	// Config is a struct, not a pointer, so it can't be nil
	// But we can check that it's not zero value
	if cfg.GRPCAddr == "" && cfg.MetricsAddr == "" {
		t.Error("DefaultConfig should not return zero value")
	}
}

func TestConfig_Struct(t *testing.T) {
	// Test that Config is a proper struct
	var cfg Config

	// Should be able to assign values
	cfg.GRPCAddr = ":50051"
	cfg.MetricsAddr = ":9090"

	if cfg.GRPCAddr != ":50051" {
		t.Error("failed to assign GRPCAddr")
	}

	if cfg.MetricsAddr != ":9090" {
		t.Error("failed to assign MetricsAddr")
	}
}

func TestConfig_Copy(t *testing.T) {
	original := DefaultConfig()
	copy := original

	// Modify copy
	copy.GRPCAddr = ":8080"

	// Original should not be affected (value semantics)
	if original.GRPCAddr != ":50051" {
		t.Error("original config was modified")
	}

	if copy.GRPCAddr != ":8080" {
		t.Error("copy was not modified")
	}
}

func TestConfig_Comparison(t *testing.T) {
	cfg1 := DefaultConfig()
	cfg2 := DefaultConfig()

	// Should be equal
	if cfg1 != cfg2 {
		t.Error("two DefaultConfig instances should be equal")
	}

	// Modify one
	cfg2.GRPCAddr = ":8080"

	// Should not be equal
	if cfg1 == cfg2 {
		t.Error("modified config should not be equal to original")
	}
}

func TestConfig_ZeroValue(t *testing.T) {
	var cfg Config

	// Zero value should have empty strings
	if cfg.GRPCAddr != "" {
		t.Errorf("zero value GRPCAddr should be empty, got %s", cfg.GRPCAddr)
	}

	if cfg.MetricsAddr != "" {
		t.Errorf("zero value MetricsAddr should be empty, got %s", cfg.MetricsAddr)
	}
}
