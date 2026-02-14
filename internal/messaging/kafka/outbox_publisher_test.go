package kafka

import (
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestOutboxPublisher_Publish(t *testing.T) {
	t.Parallel()

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndSucceed()

	producer := &Producer{
		producer: mockProducer,
		logger:   log.WithField("component", "kafka-outbox-publisher-test"),
	}
	publisher := NewOutboxPublisher(producer, TopicOrderEvents)

	err := publisher.Publish(domain.OutboxMessage{
		ID:            "outbox-1",
		AggregateType: "order",
		AggregateID:   "order-123",
		EventType:     "OrderStatusChanged",
		Payload:       []byte(`{"status":"confirmed"}`),
	})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if err := mockProducer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOutboxPublisher_PublishProducerError(t *testing.T) {
	t.Parallel()

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndFail(sarama.ErrOutOfBrokers)

	producer := &Producer{
		producer: mockProducer,
		logger:   log.WithField("component", "kafka-outbox-publisher-test"),
	}
	publisher := NewOutboxPublisher(producer, TopicOrderEvents)

	err := publisher.Publish(domain.OutboxMessage{
		ID:            "outbox-2",
		AggregateType: "order",
		AggregateID:   "order-234",
		EventType:     "OrderStatusChanged",
		Payload:       []byte(`{"status":"failed"}`),
	})
	if err == nil {
		t.Fatal("expected publish error, got nil")
	}

	if err := mockProducer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOutboxPublisher_PublishNilProducer(t *testing.T) {
	t.Parallel()

	publisher := NewOutboxPublisher(nil, TopicOrderEvents)
	if err := publisher.Publish(domain.OutboxMessage{ID: "outbox-3"}); err == nil {
		t.Fatal("expected error for nil producer")
	}
}
