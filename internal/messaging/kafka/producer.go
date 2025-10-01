package kafka

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	log "github.com/sirupsen/logrus"
)

// Producer представляет Kafka producer для публикации событий
type Producer struct {
	producer sarama.SyncProducer
	logger   *log.Entry
}

// NewProducer создает новый Kafka producer
func NewProducer(brokers []string) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Idempotent = true // Включаем идемпотентность
	config.Net.MaxOpenRequests = 1    // Для идемпотентности

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	return &Producer{
		producer: producer,
		logger:   log.WithField("component", "kafka-producer"),
	}, nil
}

// PublishEvent публикует событие в Kafka
func (p *Producer) PublishEvent(topic string, key string, event interface{}) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(key),
		Value:     sarama.ByteEncoder(eventData),
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.WithError(err).WithFields(log.Fields{
			"topic": topic,
			"key":   key,
		}).Error("failed to send message to kafka")
		return fmt.Errorf("failed to send message: %w", err)
	}

	p.logger.WithFields(log.Fields{
		"topic":     topic,
		"key":       key,
		"partition": partition,
		"offset":    offset,
	}).Debug("message sent to kafka")

	return nil
}

// Close закрывает producer
func (p *Producer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("failed to close kafka producer: %w", err)
	}
	return nil
}
