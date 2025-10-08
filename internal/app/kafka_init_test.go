package app

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestInitKafkaProducer_EmptyBrokers(t *testing.T) {
	logger := log.WithField("test", "kafka")

	producer, err := initKafkaProducer("", logger)

	if err != nil {
		t.Errorf("expected no error for empty brokers, got %v", err)
	}

	if producer != nil {
		t.Error("expected nil producer for empty brokers")
	}
}

func TestInitKafkaProducer_InvalidBrokers(t *testing.T) {
	logger := log.WithField("test", "kafka")

	// Используем несуществующий broker
	producer, err := initKafkaProducer("invalid-broker:9999", logger)

	// Должна быть ошибка, но функция продолжает работу
	if err == nil {
		t.Error("expected error for invalid brokers")
	}

	// Producer должен быть nil при ошибке
	if producer != nil {
		t.Error("expected nil producer on error")
	}
}

func TestInitKafkaProducer_MultipleBrokers(t *testing.T) {
	logger := log.WithField("test", "kafka")

	// Несколько несуществующих brokers
	brokers := "broker1:9092,broker2:9092,broker3:9092"
	producer, err := initKafkaProducer(brokers, logger)

	// Ошибка ожидается
	if err == nil {
		t.Error("expected error for invalid brokers")
	}

	if producer != nil {
		t.Error("expected nil producer on error")
	}
}

func TestCloseKafka_NilProducer(t *testing.T) {
	logger := log.WithField("test", "kafka")

	// Не должно паниковать
	closeKafka(nil, logger)
}

func TestCloseKafka_WithProducer(t *testing.T) {
	logger := log.WithField("test", "kafka")

	// Создаём producer (будет ошибка, но это ок для теста)
	producer, _ := initKafkaProducer("localhost:9999", logger)

	// Даже если producer nil, closeKafka должна работать
	closeKafka(producer, logger)
}

func TestInitKafkaProducer_BrokersWithSpaces(t *testing.T) {
	logger := log.WithField("test", "kafka")

	// Brokers с пробелами
	brokers := "broker1:9092, broker2:9092, broker3:9092"
	producer, err := initKafkaProducer(brokers, logger)

	// Ошибка ожидается (invalid brokers)
	if err == nil {
		t.Error("expected error for invalid brokers")
	}

	if producer != nil {
		t.Error("expected nil producer on error")
	}
}
