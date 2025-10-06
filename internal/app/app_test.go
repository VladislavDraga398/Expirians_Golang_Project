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
}

func TestConfig(t *testing.T) {
	cfg := Config{
		GRPCAddr:    ":8080",
		MetricsAddr: ":9091",
	}
	
	if cfg.GRPCAddr != ":8080" {
		t.Errorf("expected GRPCAddr :8080, got %s", cfg.GRPCAddr)
	}
	
	if cfg.MetricsAddr != ":9091" {
		t.Errorf("expected MetricsAddr :9091, got %s", cfg.MetricsAddr)
	}
}
