package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	log "github.com/sirupsen/logrus"
)

// MessageHandler обрабатывает сообщение из Kafka
type MessageHandler func(ctx context.Context, message *sarama.ConsumerMessage) error

// Consumer представляет Kafka consumer
type Consumer struct {
	consumer sarama.ConsumerGroup
	topics   []string
	handler  MessageHandler
	logger   *log.Entry
	wg       sync.WaitGroup
}

// NewConsumer создает новый Kafka consumer
func NewConsumer(brokers []string, groupID string, topics []string, handler MessageHandler) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer: %w", err)
	}

	return &Consumer{
		consumer: consumer,
		topics:   topics,
		handler:  handler,
		logger:   log.WithField("component", "kafka-consumer"),
	}, nil
}

// Start запускает consumer
func (c *Consumer) Start(ctx context.Context) error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			// Consume должен вызываться в цикле, так как при rebalance он завершается
			if err := c.consumer.Consume(ctx, c.topics, c); err != nil {
				c.logger.WithError(err).Error("error from consumer")
			}

			// Проверяем, не отменен ли контекст
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Обработка ошибок
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for err := range c.consumer.Errors() {
			c.logger.WithError(err).Error("consumer error")
		}
	}()

	c.logger.WithField("topics", c.topics).Info("kafka consumer started")
	return nil
}

// Stop останавливает consumer
func (c *Consumer) Stop() error {
	if err := c.consumer.Close(); err != nil {
		return fmt.Errorf("failed to close kafka consumer: %w", err)
	}
	c.wg.Wait()
	c.logger.Info("kafka consumer stopped")
	return nil
}

// Setup вызывается при старте consumer session
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup вызывается при завершении consumer session
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim обрабатывает сообщения из partition
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			c.logger.WithFields(log.Fields{
				"topic":     message.Topic,
				"partition": message.Partition,
				"offset":    message.Offset,
			}).Debug("received message")

			// Обрабатываем сообщение
			if err := c.handler(session.Context(), message); err != nil {
				c.logger.WithError(err).WithFields(log.Fields{
					"topic":     message.Topic,
					"partition": message.Partition,
					"offset":    message.Offset,
				}).Error("failed to handle message")
				// Не маркируем сообщение как обработанное при ошибке
				// В production здесь нужна логика retry или DLQ
				continue
			}

			// Маркируем сообщение как обработанное
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// ParseSagaEvent парсит SagaEvent из сообщения
func ParseSagaEvent(message *sarama.ConsumerMessage) (*SagaEvent, error) {
	var event SagaEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal saga event: %w", err)
	}
	return &event, nil
}

// ParseOrderEvent парсит OrderEvent из сообщения
func ParseOrderEvent(message *sarama.ConsumerMessage) (*OrderEvent, error) {
	var event OrderEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order event: %w", err)
	}
	return &event, nil
}
