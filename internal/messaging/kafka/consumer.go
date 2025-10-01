package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
	log "github.com/sirupsen/logrus"
)

// MessageHandler обрабатывает сообщение из Kafka
type MessageHandler func(ctx context.Context, message *sarama.ConsumerMessage) error

// Consumer представляет Kafka consumer с поддержкой DLQ
type Consumer struct {
	consumer    sarama.ConsumerGroup
	topics      []string
	handler     MessageHandler
	logger      *log.Entry
	wg          sync.WaitGroup
	dlqProducer *Producer      // Producer для отправки в DLQ
	maxRetries  int            // Максимальное количество retry попыток
}

// NewConsumer создает новый Kafka consumer
func NewConsumer(brokers []string, groupID string, topics []string, handler MessageHandler) (*Consumer, error) {
	return NewConsumerWithDLQ(brokers, groupID, topics, handler, nil, 3)
}

// NewConsumerWithDLQ создает consumer с поддержкой Dead Letter Queue
func NewConsumerWithDLQ(brokers []string, groupID string, topics []string, handler MessageHandler, dlqProducer *Producer, maxRetries int) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka consumer: %w", err)
	}

	return &Consumer{
		consumer:    consumer,
		topics:      topics,
		handler:     handler,
		logger:      log.WithField("component", "kafka-consumer"),
		dlqProducer: dlqProducer,
		maxRetries:  maxRetries,
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

			// Обрабатываем сообщение с retry и DLQ логикой
			if err := c.handleMessageWithRetry(session.Context(), message); err != nil {
				c.logger.WithError(err).WithFields(log.Fields{
					"topic":     message.Topic,
					"partition": message.Partition,
					"offset":    message.Offset,
				}).Error("message processing failed after all retries")
				// Не маркируем сообщение - оно уже в DLQ или будет reprocessed
				continue
			}

			// Маркируем сообщение как обработанное
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// handleMessageWithRetry обрабатывает сообщение с retry логикой и отправкой в DLQ
func (c *Consumer) handleMessageWithRetry(ctx context.Context, message *sarama.ConsumerMessage) error {
	// Получаем текущий retry count из headers
	retryCount := c.getRetryCount(message)
	
	// Пытаемся обработать сообщение
	err := c.handler(ctx, message)
	if err == nil {
		return nil // Успешно обработано
	}
	
	// Если ошибка и не достигнут лимит retry
	if retryCount < c.maxRetries {
		c.logger.WithFields(log.Fields{
			"topic":       message.Topic,
			"retry_count": retryCount,
			"max_retries": c.maxRetries,
		}).Warn("message processing failed, will retry")
		
		// В реальной системе здесь можно добавить exponential backoff
		// или отправить в retry topic с delay
		return err
	}
	
	// Исчерпаны все попытки - отправляем в DLQ
	if c.dlqProducer != nil {
		if dlqErr := c.sendToDLQ(message, err); dlqErr != nil {
			c.logger.WithError(dlqErr).Error("failed to send message to DLQ")
			return fmt.Errorf("failed to send to DLQ: %w", dlqErr)
		}
		c.logger.WithFields(log.Fields{
			"topic":       message.Topic,
			"retry_count": retryCount,
		}).Info("message sent to DLQ after max retries")
		return nil // Считаем обработанным, так как отправили в DLQ
	}
	
	return err
}

// getRetryCount извлекает retry count из headers сообщения
func (c *Consumer) getRetryCount(message *sarama.ConsumerMessage) int {
	for _, header := range message.Headers {
		if string(header.Key) == HeaderRetryCount {
			count, err := strconv.Atoi(string(header.Value))
			if err == nil {
				return count
			}
		}
	}
	return 0
}

// sendToDLQ отправляет failed message в Dead Letter Queue
func (c *Consumer) sendToDLQ(message *sarama.ConsumerMessage, processingErr error) error {
	// Создаём DLQ message с дополнительными headers
	dlqMessage := map[string]interface{}{
		"original_topic":     message.Topic,
		"original_partition": message.Partition,
		"original_offset":    message.Offset,
		"original_key":       string(message.Key),
		"original_value":     string(message.Value),
		"error_message":      processingErr.Error(),
		"failed_at":          time.Now().UTC().Format(time.RFC3339),
		"retry_count":        c.getRetryCount(message),
	}
	
	// Отправляем в DLQ topic
	return c.dlqProducer.PublishEvent(
		TopicDeadLetterQueue,
		string(message.Key),
		dlqMessage,
	)
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
