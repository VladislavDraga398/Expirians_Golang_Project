package kafka

import (
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	log "github.com/sirupsen/logrus"
)

func TestProducer_PublishEvent(t *testing.T) {
	// Создаем mock producer
	mockProducer := mocks.NewSyncProducer(t, nil)
	
	producer := &Producer{
		producer: mockProducer,
		logger:   log.WithField("component", "kafka-producer-test"),
	}

	// Настраиваем ожидания
	mockProducer.ExpectSendMessageAndSucceed()

	// Создаем тестовое событие
	event := NewSagaEvent(
		EventTypeSagaStarted,
		"test-order-123",
		map[string]interface{}{
			"customer_id": "cust-1",
		},
	)

	// Публикуем событие
	err := producer.PublishEvent(TopicSagaEvents, "test-order-123", event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Проверяем, что все ожидания выполнены
	if err := mockProducer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProducer_PublishEvent_Error(t *testing.T) {
	// Создаем mock producer с ошибкой
	mockProducer := mocks.NewSyncProducer(t, nil)
	
	producer := &Producer{
		producer: mockProducer,
		logger:   log.WithField("component", "kafka-producer-test"),
	}

	// Настраиваем ожидание ошибки
	mockProducer.ExpectSendMessageAndFail(sarama.ErrOutOfBrokers)

	event := NewSagaEvent(
		EventTypeSagaStarted,
		"test-order-123",
		nil,
	)

	// Публикуем событие
	err := producer.PublishEvent(TopicSagaEvents, "test-order-123", event)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mockProducer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewSagaEvent(t *testing.T) {
	orderID := "order-123"
	metadata := map[string]interface{}{
		"customer_id": "cust-1",
		"amount":      1000,
	}

	event := NewSagaEvent(EventTypeSagaStarted, orderID, metadata)

	if event.EventType != EventTypeSagaStarted {
		t.Errorf("expected event type %s, got %s", EventTypeSagaStarted, event.EventType)
	}

	if event.OrderID != orderID {
		t.Errorf("expected order id %s, got %s", orderID, event.OrderID)
	}

	if event.Metadata["customer_id"] != "cust-1" {
		t.Error("metadata not set correctly")
	}

	// Проверяем, что timestamp установлен
	if event.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}

	// Проверяем, что timestamp близок к текущему времени
	if time.Since(event.Timestamp) > time.Second {
		t.Error("timestamp should be close to current time")
	}
}

func TestNewOrderEvent(t *testing.T) {
	orderID := "order-123"
	customerID := "cust-1"
	status := "confirmed"
	metadata := map[string]interface{}{
		"amount": 1000,
	}

	event := NewOrderEvent(EventTypeOrderConfirmed, orderID, customerID, status, metadata)

	if event.EventType != EventTypeOrderConfirmed {
		t.Errorf("expected event type %s, got %s", EventTypeOrderConfirmed, event.EventType)
	}

	if event.OrderID != orderID {
		t.Errorf("expected order id %s, got %s", orderID, event.OrderID)
	}

	if event.CustomerID != customerID {
		t.Errorf("expected customer id %s, got %s", customerID, event.CustomerID)
	}

	if event.Status != status {
		t.Errorf("expected status %s, got %s", status, event.Status)
	}

	if event.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}
