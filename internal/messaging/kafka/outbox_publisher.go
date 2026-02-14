package kafka

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

// OutboxTopicPublisher публикует outbox-сообщения в заданный Kafka topic.
type OutboxTopicPublisher struct {
	producer *Producer
	topic    string
}

// NewOutboxPublisher создаёт Kafka-паблишер для transactional outbox.
func NewOutboxPublisher(producer *Producer, topic string) domain.OutboxPublisher {
	if topic == "" {
		topic = TopicOrderEvents
	}
	return &OutboxTopicPublisher{
		producer: producer,
		topic:    topic,
	}
}

func (p *OutboxTopicPublisher) Publish(event domain.OutboxMessage) error {
	if p == nil || p.producer == nil {
		return fmt.Errorf("kafka outbox publisher is not initialized")
	}

	key := event.AggregateID
	if key == "" {
		key = event.ID
	}

	envelope := struct {
		ID            string          `json:"id"`
		AggregateType string          `json:"aggregate_type"`
		AggregateID   string          `json:"aggregate_id"`
		EventType     string          `json:"event_type"`
		Payload       json.RawMessage `json:"payload"`
		PublishedAt   time.Time       `json:"published_at"`
	}{
		ID:            event.ID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		Payload:       json.RawMessage(event.Payload),
		PublishedAt:   time.Now().UTC(),
	}

	return p.producer.PublishEvent(p.topic, key, envelope)
}

var _ domain.OutboxPublisher = (*OutboxTopicPublisher)(nil)
