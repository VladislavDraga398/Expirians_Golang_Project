package app

import (
	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	"github.com/vladislavdragonenkov/oms/internal/service/inventory"
	"github.com/vladislavdragonenkov/oms/internal/service/payment"
	"github.com/vladislavdragonenkov/oms/internal/storage/memory"
)

// Dependencies содержит все зависимости приложения.
type Dependencies struct {
	Repo         domain.OrderRepository
	OutboxRepo   domain.OutboxRepository
	TimelineRepo domain.TimelineRepository
	InventorySvc domain.InventoryService
	PaymentSvc   domain.PaymentService
	Logger       *log.Entry
}

// NewDependencies создаёт зависимости для локального запуска (in-memory + mock сервисы).
func NewDependencies(logger *log.Entry) *Dependencies {
	if logger == nil {
		logger = log.WithField("component", "app")
	}

	return &Dependencies{
		Repo:         memory.NewOrderRepository(),
		OutboxRepo:   memory.NewOutboxRepository(),
		TimelineRepo: memory.NewTimelineRepository(),
		InventorySvc: inventory.NewMockService(),
		PaymentSvc:   payment.NewMockService(),
		Logger:       logger,
	}
}
