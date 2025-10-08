package app

import (
	"github.com/vladislavdragonenkov/oms/internal/messaging/kafka"
	"github.com/vladislavdragonenkov/oms/internal/service/saga"
)

// createOrchestrator создаёт saga orchestrator с или без Kafka в зависимости
// от наличия kafka producer.
func createOrchestrator(
	deps *Dependencies,
	kafkaProducer *kafka.Producer,
) saga.Orchestrator {
	if kafkaProducer != nil {
		return saga.NewOrchestratorWithKafka(
			deps.Repo,
			deps.OutboxRepo,
			deps.TimelineRepo,
			deps.InventorySvc,
			deps.PaymentSvc,
			kafkaProducer,
			deps.Logger,
		)
	}

	return saga.NewOrchestrator(
		deps.Repo,
		deps.OutboxRepo,
		deps.TimelineRepo,
		deps.InventorySvc,
		deps.PaymentSvc,
		deps.Logger,
	)
}
