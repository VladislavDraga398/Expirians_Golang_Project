# Kafka Integration Guide

## Обзор

В Phase 4 мы добавили **Apache Kafka** для реализации event-driven архитектуры. Это позволяет:
- Асинхронную обработку saga событий
- Масштабируемость через consumer groups
- Гарантии доставки (at-least-once)
- Retry механизмы и error handling
- Audit trail всех событий

## Архитектура

```
┌─────────────┐      ┌──────────┐      ┌─────────────┐
│ OMS Service │─────>│  Kafka   │─────>│  Consumers  │
│  (Producer) │      │  Broker  │      │   (Groups)  │
└─────────────┘      └──────────┘      └─────────────┘
                           │
                           ├─ Topic: oms.saga.events
                           └─ Topic: oms.order.events
```

## Компоненты

### 1. Kafka Broker
- **Image**: `confluentinc/cp-kafka:7.5.0`
- **Ports**: 
  - `9092` - внутренний (для контейнеров)
  - `9093` - внешний (для localhost)

### 2. Zookeeper
- **Image**: `confluentinc/cp-zookeeper:7.5.0`
- **Port**: `2181`

### 3. Kafka UI
- **Image**: `provectuslabs/kafka-ui:latest`
- **Port**: `8080`
- **URL**: http://localhost:8080

## Topics

### `oms.saga.events`
События жизненного цикла саги:
- `saga.started` - сага запущена
- `saga.completed` - сага успешно завершена
- `saga.failed` - сага провалилась
- `saga.canceled` - сага отменена
- `saga.refunded` - выполнен возврат

### `oms.order.events`
События заказа:
- `order.created` - заказ создан
- `order.confirmed` - заказ подтвержден
- `order.canceled` - заказ отменен
- `order.refunded` - возврат выполнен

## Запуск

### 1. Запуск всего стека
```bash
docker compose up -d
```

### 2. Проверка здоровья Kafka
```bash
docker compose logs kafka | grep "started (kafka.server.KafkaServer)"
```

### 3. Открыть Kafka UI
```bash
open http://localhost:8080
```

## Использование

### Producer (публикация событий)

```go
import "github.com/vladislavdragonenkov/oms/internal/messaging/kafka"

// Создание producer
producer, err := kafka.NewProducer([]string{"localhost:9093"})
if err != nil {
    log.Fatal(err)
}
defer producer.Close()

// Публикация события
event := kafka.NewSagaEvent(
    kafka.EventTypeSagaStarted,
    orderID,
    map[string]interface{}{
        "customer_id": customerID,
    },
)

err = producer.PublishEvent(kafka.TopicSagaEvents, orderID, event)
```

### Consumer (обработка событий)

```go
// Обработчик сообщений
handler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
    event, err := kafka.ParseSagaEvent(msg)
    if err != nil {
        return err
    }
    
    log.Printf("Received event: %s for order %s", event.EventType, event.OrderID)
    return nil
}

// Создание consumer
consumer, err := kafka.NewConsumer(
    []string{"localhost:9093"},
    "oms-saga-consumer",
    []string{kafka.TopicSagaEvents},
    handler,
)

// Запуск
ctx := context.Background()
consumer.Start(ctx)
```

## Мониторинг

### Метрики Kafka
- **Producer throughput**: сообщений/сек
- **Consumer lag**: отставание consumer от producer
- **Error rate**: процент ошибок

### Grafana Dashboard
Добавлены панели для мониторинга Kafka:
- Message rate
- Consumer lag
- Error rate
- Topic size

## Troubleshooting

### Kafka не стартует
```bash
# Проверить логи
docker compose logs kafka

# Проверить Zookeeper
docker compose logs zookeeper

# Перезапустить
docker compose restart kafka
```

### Consumer не получает сообщения
```bash
# Проверить consumer group
docker compose exec kafka kafka-consumer-groups \
  --bootstrap-server localhost:9092 \
  --group oms-saga-consumer \
  --describe
```

### Очистить все данные
```bash
docker compose down -v
docker compose up -d
```

## Best Practices

### 1. Идемпотентность
Producer настроен с `Idempotent=true` для предотвращения дубликатов.

### 2. Retry Policy
- Producer: 5 попыток с exponential backoff
- Consumer: обработка ошибок с логированием

### 3. Compression
Используется Snappy compression для уменьшения размера сообщений.

### 4. Partitioning
Ключ партиционирования = `order_id` для гарантии порядка событий одного заказа.

## Следующие шаги

- [ ] Добавить Dead Letter Queue (DLQ) для failed messages
- [ ] Реализовать Schema Registry для версионирования событий
- [ ] Добавить distributed tracing (trace context в headers)
- [ ] Настроить alerting на consumer lag
