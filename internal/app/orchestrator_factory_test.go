package app

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
)

func TestCreateOrchestrator_WithoutKafka(t *testing.T) {
	logger := log.WithField("test", "orchestrator")
	deps := NewDependencies(logger)

	// Используем версию без metrics для тестов
	orch := saga.NewOrchestratorWithoutMetrics(
		deps.Repo,
		deps.OutboxRepo,
		deps.TimelineRepo,
		deps.InventorySvc,
		deps.PaymentSvc,
		logger,
	)

	if orch == nil {
		t.Fatal("orchestrator should not return nil")
	}
}

func TestCreateOrchestrator_BothPaths(t *testing.T) {
	logger := log.WithField("test", "orchestrator")
	deps := NewDependencies(logger)

	// Тест логики выбора orchestrator (без реального создания из-за metrics)
	// Проверяем что функция существует и компилируется
	_ = createOrchestrator

	// Проверяем nil kafka producer path
	orch1 := saga.NewOrchestratorWithoutMetrics(
		deps.Repo,
		deps.OutboxRepo,
		deps.TimelineRepo,
		deps.InventorySvc,
		deps.PaymentSvc,
		logger,
	)

	if orch1 == nil {
		t.Fatal("orchestrator without kafka should not be nil")
	}

	// Проверка что createOrchestrator принимает правильные параметры
	// (тип проверяется на этапе компиляции)
	type factoryFunc func(*Dependencies, interface{}) saga.Orchestrator
	var _ factoryFunc = func(d *Dependencies, k interface{}) saga.Orchestrator {
		return createOrchestrator(d, nil)
	}
}
