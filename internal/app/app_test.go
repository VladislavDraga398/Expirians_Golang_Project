package app

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.GRPCAddr == "" {
		t.Error("GRPCAddr should not be empty")
	}

	if cfg.MetricsAddr == "" {
		t.Error("MetricsAddr should not be empty")
	}

	// Test default values
	if cfg.GRPCAddr != ":50051" {
		t.Errorf("expected GRPCAddr :50051, got %s", cfg.GRPCAddr)
	}

	if cfg.MetricsAddr != ":9090" {
		t.Errorf("expected MetricsAddr :9090, got %s", cfg.MetricsAddr)
	}

	if cfg.StorageDriver != StorageDriverMemory {
		t.Errorf("expected StorageDriver %s, got %s", StorageDriverMemory, cfg.StorageDriver)
	}
}

func TestConfig(t *testing.T) {
	cfg := Config{
		GRPCAddr:            ":8080",
		MetricsAddr:         ":9091",
		StorageDriver:       StorageDriverPostgres,
		PostgresDSN:         "postgres://oms:oms@localhost:5432/oms?sslmode=disable",
		PostgresAutoMigrate: false,
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
}

func TestParseKafkaBrokers(t *testing.T) {
	brokers := parseKafkaBrokers("broker-1:9092, broker-2:9092,, broker-3:9092 ")

	if len(brokers) != 3 {
		t.Fatalf("expected 3 brokers, got %d", len(brokers))
	}

	if brokers[0] != "broker-1:9092" {
		t.Fatalf("unexpected first broker: %s", brokers[0])
	}
	if brokers[1] != "broker-2:9092" {
		t.Fatalf("unexpected second broker: %s", brokers[1])
	}
	if brokers[2] != "broker-3:9092" {
		t.Fatalf("unexpected third broker: %s", brokers[2])
	}
}

func TestParseKafkaBrokers_Empty(t *testing.T) {
	brokers := parseKafkaBrokers(" , , ")
	if len(brokers) != 0 {
		t.Fatalf("expected no brokers, got %d", len(brokers))
	}
}
