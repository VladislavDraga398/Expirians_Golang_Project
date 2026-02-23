# Kafka Integration Guide

Интеграция Kafka в текущем OMS runtime.

**Версия:** v2.2 | **Обновлено:** 2026-02-23 | **Статус:** Актуально

---

## TL;DR
- Kafka используется для публикации outbox-событий и DLQ.
- Основные топики: `oms.order.events`, `oms.saga.events`, `oms.dlq`.
- Producer настроен с `acks=all`, `idempotent=true`, `snappy`.
- Consumer поддерживает локальные retry-попытки и отправку в DLQ при исчерпании попыток.

## Компоненты (docker-compose)

### Kafka
- Image: `confluentinc/cp-kafka:7.5.0`
- Порты:
  - `9092` (внутри docker-сети)
  - `9093` (localhost)

### Zookeeper
- Image: `confluentinc/cp-zookeeper:7.5.0`
- Порт: `2181`

### Kafka UI
- Image: `provectuslabs/kafka-ui@sha256:8f2ff02d64b0a7a2b71b6b3b3148b85f66d00ec20ad40c30bdcd415d46d31818`
- URL: http://localhost:8080

## Топики
- `oms.order.events` — события заказа из outbox publisher.
- `oms.saga.events` — saga lifecycle events.
- `oms.dlq` — сообщения, не прошедшие обработку после retry.

## Runtime поток публикации
1. В транзакции записывается бизнес-изменение + запись в `outbox_messages`.
2. Outbox worker забирает batch через claim (`FOR UPDATE SKIP LOCKED`).
3. Публикует событие в Kafka.
4. Помечает запись как `sent` или `failed`; при исчерпании попыток отправляет в DLQ.

## Конфигурация
- `KAFKA_BROKERS` — список брокеров через запятую.
- При пустом `KAFKA_BROKERS` сервис работает без Kafka producer.
- При невалидном значении `KAFKA_BROKERS` runtime завершается с ошибкой конфигурации.

## Проверка локально

```bash
# Поднять стек
make compose-up

# Проверить broker логи
docker compose logs kafka | rg "started \(kafka.server.KafkaServer\)"

# Открыть UI
open http://localhost:8080
```

## Troubleshooting

### Kafka не стартует
```bash
docker compose logs zookeeper
docker compose logs kafka
docker compose restart zookeeper kafka
```

### Нет событий в `oms.order.events`
```bash
# Проверить OMS env
printenv KAFKA_BROKERS

# Проверить outbox backlog в метриках/логах
# и наличие записей pending/processing в outbox_messages
```

### Рост `oms.dlq`
- Проверить причину в payload DLQ-сообщений.
- Сделать controlled replay через `make dlq-reprocess`.
- См. runbook: `docs/operations/runbooks.md`.

## Что в roadmap дальше
- Добавить alerting по consumer lag и DLQ burst.
- Добавить trace-context propagation в headers Kafka сообщений.
- Добавить policy для replay/retention per-topic.
