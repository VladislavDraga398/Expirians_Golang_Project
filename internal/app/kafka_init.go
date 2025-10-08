package app

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/vladislavdragonenkov/oms/internal/messaging/kafka"
)

// initKafkaProducer инициализирует Kafka producer если brokers не пустой.
// Возвращает nil, nil если brokers пустой или если произошла ошибка.
func initKafkaProducer(brokers string, logger *log.Entry) (*kafka.Producer, error) {
	if brokers == "" {
		return nil, nil
	}

	brokerList := strings.Split(brokers, ",")
	producer, err := kafka.NewProducer(brokerList)
	if err != nil {
		logger.WithError(err).Warn("failed to create kafka producer, continuing without kafka")
		return nil, err
	}

	logger.WithField("brokers", brokerList).Info("kafka producer initialized")
	return producer, nil
}

// closeKafka закрывает Kafka producer если он не nil.
func closeKafka(producer *kafka.Producer, logger *log.Entry) {
	if producer == nil {
		return
	}

	if err := producer.Close(); err != nil {
		logger.WithError(err).Warn("failed to close kafka producer")
	} else {
		logger.Info("kafka producer closed")
	}
}
