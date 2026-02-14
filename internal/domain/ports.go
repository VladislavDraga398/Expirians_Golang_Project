package domain

import "time"

// InventoryService описывает взаимодействие с сервисом складских резервов.
type InventoryService interface {
	// Reserve пытается зарезервировать товары под заказ.
	Reserve(orderID string, items []OrderItem) error
	// Release снимает резерв по заказу (компенсация).
	Release(orderID string, items []OrderItem) error
}

// PaymentService описывает взаимодействие с платёжным провайдером.
type PaymentService interface {
	// Pay инициирует списание средств по заказу.
	Pay(orderID string, amountMinor int64, currency string) (PaymentStatus, error)
	// Refund инициирует возврат средств (для компенсаций/отмен).
	Refund(orderID string, amountMinor int64, currency string) (PaymentStatus, error)
}

// OutboxPublisher публикует события из transactional outbox.
type OutboxPublisher interface {
	// Publish передаёт событие наружу; должен быть идемпотентным.
	Publish(event OutboxMessage) error
}

// OutboxRepository позволяет сохранять события для последующей публикации.
type OutboxRepository interface {
	Enqueue(msg OutboxMessage) (OutboxMessage, error)
	PullPending(limit int) ([]OutboxMessage, error)
	Stats() (OutboxStats, error)
	MarkSent(id string) error
	MarkFailed(id string) error
}

// TimelineRepository хранит события жизненного цикла заказа.
type TimelineRepository interface {
	Append(event TimelineEvent) error
	List(orderID string) ([]TimelineEvent, error)
}

// IdempotencyRepository хранит состояние обработки запросов по idempotency-key.
type IdempotencyRepository interface {
	CreateProcessing(key, requestHash string, ttlAt time.Time) (IdempotencyRecord, error)
	Get(key string) (IdempotencyRecord, error)
	MarkDone(key string, responseBody []byte, httpStatus int) error
	MarkFailed(key string, responseBody []byte, httpStatus int) error
	DeleteExpired(before time.Time, limit int) (int, error)
}

// SagaStep задаёт константы шагов для метрик/логов.
type SagaStep string

const (
	SagaStepReserve SagaStep = "reserve"
	SagaStepPay     SagaStep = "pay"
	SagaStepConfirm SagaStep = "confirm"
	SagaStepRelease SagaStep = "release"
	SagaStepCancel  SagaStep = "cancel"
	SagaStepRefund  SagaStep = "refund"
)

// OutboxMessage хранит данные для публикуемого события.
type OutboxMessage struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
}

// OutboxStats описывает текущее состояние backlog transactional outbox.
type OutboxStats struct {
	PendingCount    int
	OldestPendingAt time.Time
}
