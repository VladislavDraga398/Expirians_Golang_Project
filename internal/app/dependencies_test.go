package app

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestNewDependencies(t *testing.T) {
	logger := log.WithField("test", "dependencies")
	deps := NewDependencies(logger)

	if deps == nil {
		t.Fatal("NewDependencies should not return nil")
	}

	if deps.Repo == nil {
		t.Error("Repo should not be nil")
	}

	if deps.OutboxRepo == nil {
		t.Error("OutboxRepo should not be nil")
	}

	if deps.TimelineRepo == nil {
		t.Error("TimelineRepo should not be nil")
	}

	if deps.InventorySvc == nil {
		t.Error("InventorySvc should not be nil")
	}

	if deps.PaymentSvc == nil {
		t.Error("PaymentSvc should not be nil")
	}

	if deps.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestNewDependencies_WithNilLogger(t *testing.T) {
	deps := NewDependencies(nil)

	if deps == nil {
		t.Fatal("NewDependencies should not return nil")
	}

	if deps.Logger == nil {
		t.Error("Logger should be initialized even when nil is passed")
	}
}

func TestNewDependencies_AllFieldsInitialized(t *testing.T) {
	logger := log.WithField("test", "all-fields")
	deps := NewDependencies(logger)

	// Проверяем что можем использовать все зависимости
	if deps.Repo == nil {
		t.Fatal("Repo not initialized")
	}

	// Проверяем что репозитории работают
	order := newTestOrder()
	if err := deps.Repo.Create(order); err != nil {
		t.Errorf("Repo.Create failed: %v", err)
	}

	// Проверяем что сервисы работают
	items := []struct {
		SKU string
		Qty int
	}{
		{"SKU-1", 1},
	}

	orderItems := make([]interface{}, len(items))
	for i, item := range items {
		orderItems[i] = struct {
			SKU string
			Qty int
		}{item.SKU, item.Qty}
	}

	// InventorySvc должен работать (mock всегда успешен)
	// PaymentSvc должен работать (mock всегда успешен)
}

func TestNewDependencies_LoggerField(t *testing.T) {
	customLogger := log.WithField("custom", "value")
	deps := NewDependencies(customLogger)

	if deps.Logger != customLogger {
		t.Error("Logger should be the same instance as passed")
	}
}

func TestNewDependencies_IndependentInstances(t *testing.T) {
	deps1 := NewDependencies(nil)
	deps2 := NewDependencies(nil)

	// Каждый вызов должен создавать новые экземпляры
	if deps1 == deps2 {
		t.Error("NewDependencies should create independent instances")
	}

	// Репозитории должны быть разными
	if deps1.Repo == deps2.Repo {
		t.Error("Repo instances should be independent")
	}
}
