package kafka

import "time"

// EventType определяет тип события
type EventType string

const (
	// Saga события
	EventTypeSagaStarted   EventType = "saga.started"
	EventTypeSagaCompleted EventType = "saga.completed"
	EventTypeSagaFailed    EventType = "saga.failed"
	EventTypeSagaCanceled  EventType = "saga.canceled"
	EventTypeSagaRefunded  EventType = "saga.refunded"

	// Order события
	EventTypeOrderCreated   EventType = "order.created"
	EventTypeOrderConfirmed EventType = "order.confirmed"
	EventTypeOrderCanceled  EventType = "order.canceled"
	EventTypeOrderRefunded  EventType = "order.refunded"

	// Step события
	EventTypeStepReserved EventType = "step.reserved"
	EventTypeStepPaid     EventType = "step.paid"
)

// Topics для Kafka
const (
	TopicSagaEvents      = "oms.saga.events"
	TopicOrderEvents     = "oms.order.events"
	TopicDeadLetterQueue = "oms.dlq" // Dead Letter Queue для failed messages
)

// Kafka headers для retry логики
const (
	HeaderRetryCount    = "x-retry-count"
	HeaderOriginalTopic = "x-original-topic"
	HeaderErrorMessage  = "x-error-message"
	HeaderFailedAt      = "x-failed-at"
)

// SagaEvent представляет событие саги
type SagaEvent struct {
	EventType EventType              `json:"event_type"`
	OrderID   string                 `json:"order_id"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// OrderEvent представляет событие заказа
type OrderEvent struct {
	EventType  EventType              `json:"event_type"`
	OrderID    string                 `json:"order_id"`
	CustomerID string                 `json:"customer_id"`
	Status     string                 `json:"status"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NewSagaEvent создает новое событие саги
func NewSagaEvent(eventType EventType, orderID string, metadata map[string]interface{}) *SagaEvent {
	return &SagaEvent{
		EventType: eventType,
		OrderID:   orderID,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
}

// NewOrderEvent создает новое событие заказа
func NewOrderEvent(eventType EventType, orderID, customerID, status string, metadata map[string]interface{}) *OrderEvent {
	return &OrderEvent{
		EventType:  eventType,
		OrderID:    orderID,
		CustomerID: customerID,
		Status:     status,
		Timestamp:  time.Now(),
		Metadata:   metadata,
	}
}
