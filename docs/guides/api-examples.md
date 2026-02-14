# API Examples

Полное руководство по использованию OMS API с примерами для всех операций.

## Быстрый старт

```bash
# Убедитесь, что сервис запущен
make run

# Или запустите полное демо
make demo
```

---

## Содержание

- [CreateOrder](#createorder) - Создание заказа
- [GetOrder](#getorder) - Получение заказа
- [PayOrder](#payorder) - Оплата заказа
- [CancelOrder](#cancelorder) - Отмена заказа
- [RefundOrder](#refundorder) - Возврат средств
- [Сценарии использования](#сценарии-использования)
- [Обработка ошибок](#обработка-ошибок)

---

## CreateOrder

Создание нового заказа.

### gRPC (grpcurl)

```bash
grpcurl -plaintext -d '{
  "customer_id": "customer-123",
  "currency": "USD",
  "items": [
    {
      "sku": "SKU-001",
      "qty": 2,
      "price": {
        "currency": "USD",
        "amount_minor": 10000
      }
    },
    {
      "sku": "SKU-002",
      "qty": 1,
      "price": {
        "currency": "USD",
        "amount_minor": 5000
      }
    }
  ]
}' localhost:50051 oms.v1.OrderService/CreateOrder
```

### Ответ

```json
{
  "order_id": "01HQZX...",
  "status": "ORDER_STATUS_PENDING",
  "created_at": "2025-10-01T14:30:00Z"
}
```

### Параметры

| Поле | Тип | Обязательно | Описание |
|------|-----|-------------|----------|
| `customer_id` | string |  | ID клиента |
| `currency` | string |  | Код валюты (USD, EUR, RUB) |
| `items` | array |  | Список товаров (минимум 1) |
| `items[].sku` | string |  | Артикул товара |
| `items[].qty` | int32 |  | Количество (> 0) |
| `items[].price.currency` | string |  | Валюта цены |
| `items[].price.amount_minor` | int64 |  | Цена в минимальных единицах (копейки) |

### Идемпотентность

На текущем этапе `CreateOrder` создаёт новый заказ при каждом вызове. Полная идемпотентность через `idempotency-key` находится в roadmap.

---

## GetOrder

Получение информации о заказе.

### gRPC (grpcurl)

```bash
grpcurl -plaintext -d '{
  "order_id": "01HQZX..."
}' localhost:50051 oms.v1.OrderService/GetOrder
```

### Ответ

```json
{
  "order": {
    "id": "01HQZX...",
    "customer_id": "customer-123",
    "status": "ORDER_STATUS_PENDING",
    "currency": "USD",
    "amount_minor": 25000,
    "items": [
      {
        "id": "item-1",
        "sku": "SKU-001",
        "qty": 2,
        "price_minor": 10000,
        "created_at": "2025-10-01T14:30:00Z"
      }
    ],
    "version": 1,
    "created_at": "2025-10-01T14:30:00Z",
    "updated_at": "2025-10-01T14:30:00Z"
  },
  "timeline": [
    {
      "event_type": "OrderCreated",
      "timestamp": "2025-10-01T14:30:00Z",
      "payload": "{\"customer_id\":\"customer-123\"}"
    }
  ]
}
```

### Статусы заказа

| Статус | Описание |
|--------|----------|
| `ORDER_STATUS_PENDING` | Заказ создан, ожидает обработки |
| `ORDER_STATUS_RESERVED` | Товары зарезервированы на складе |
| `ORDER_STATUS_PAID` | Оплата подтверждена |
| `ORDER_STATUS_CONFIRMED` | Заказ подтверждён и готов к исполнению |
| `ORDER_STATUS_CANCELED` | Заказ отменён |
| `ORDER_STATUS_REFUNDED` | Средства возвращены |

---

## PayOrder

Оплата заказа. Запускает Saga: Reserve → Pay → Confirm.

### gRPC (grpcurl)

```bash
grpcurl -plaintext -d '{
  "order_id": "01HQZX..."
}' localhost:50051 oms.v1.OrderService/PayOrder
```

### Ответ

```json
{
  "order_id": "01HQZX...",
  "status": "ORDER_STATUS_PENDING"
}
```

### Процесс

1. **Reserve** - резервирование товаров на складе
2. **Pay** - списание средств с клиента
3. **Confirm** - подтверждение заказа

Saga выполняется асинхронно. Проверьте статус через `GetOrder`.

### Компенсации

При ошибке на любом шаге:
- Если Reserve failed → статус `CANCELED`
- Если Pay failed → Release inventory → статус `CANCELED`

---

## CancelOrder

Отмена заказа с компенсациями.

### gRPC (grpcurl)

```bash
grpcurl -plaintext -d '{
  "order_id": "01HQZX...",
  "reason": "Customer request"
}' localhost:50051 oms.v1.OrderService/CancelOrder
```

### Ответ

```json
{
  "order_id": "01HQZX...",
  "status": "ORDER_STATUS_CANCELED"
}
```

### Компенсации

В зависимости от текущего статуса:
- `RESERVED` → Release inventory
- `PAID` → Refund payment + Release inventory
- `CONFIRMED` → Refund payment + Release inventory

---

## RefundOrder

Возврат средств (полный или частичный).

### gRPC (grpcurl)

#### Полный возврат

```bash
grpcurl -plaintext -d '{
  "order_id": "01HQZX..."
}' localhost:50051 oms.v1.OrderService/RefundOrder
```

#### Частичный возврат

```bash
grpcurl -plaintext -d '{
  "order_id": "01HQZX...",
  "amount": {
    "currency": "USD",
    "amount_minor": 10000
  },
  "reason": "Damaged item"
}' localhost:50051 oms.v1.OrderService/RefundOrder
```

### Ответ

```json
{
  "order_id": "01HQZX...",
  "status": "ORDER_STATUS_REFUNDED",
  "refunded_amount": 10000
}
```

---

## Сценарии использования

### Сценарий 1: Успешный заказ

```bash
# 1. Создать заказ
ORDER_ID=$(grpcurl -plaintext -d '{
  "customer_id": "customer-123",
  "currency": "USD",
  "items": [{"sku": "SKU-001", "qty": 1, "price": {"currency": "USD", "amount_minor": 10000}}]
}' localhost:50051 oms.v1.OrderService/CreateOrder | jq -r '.order_id')

echo "Order ID: $ORDER_ID"

# 2. Оплатить заказ
grpcurl -plaintext -d "{\"order_id\": \"$ORDER_ID\"}" \
  localhost:50051 oms.v1.OrderService/PayOrder

# 3. Подождать завершения саги
sleep 2

# 4. Проверить статус
grpcurl -plaintext -d "{\"order_id\": \"$ORDER_ID\"}" \
  localhost:50051 oms.v1.OrderService/GetOrder | jq '.order.status'

# Ожидаемый результат: "ORDER_STATUS_CONFIRMED"
```

### Сценарий 2: Отмена заказа

```bash
# 1. Создать и оплатить заказ
ORDER_ID=$(grpcurl -plaintext -d '{
  "customer_id": "customer-456",
  "currency": "USD",
  "items": [{"sku": "SKU-002", "qty": 2, "price": {"currency": "USD", "amount_minor": 5000}}]
}' localhost:50051 oms.v1.OrderService/CreateOrder | jq -r '.order_id')

grpcurl -plaintext -d "{\"order_id\": \"$ORDER_ID\"}" \
  localhost:50051 oms.v1.OrderService/PayOrder

sleep 2

# 2. Отменить заказ
grpcurl -plaintext -d "{\"order_id\": \"$ORDER_ID\", \"reason\": \"Customer changed mind\"}" \
  localhost:50051 oms.v1.OrderService/CancelOrder

# 3. Проверить статус
grpcurl -plaintext -d "{\"order_id\": \"$ORDER_ID\"}" \
  localhost:50051 oms.v1.OrderService/GetOrder | jq '.order.status'

# Ожидаемый результат: "ORDER_STATUS_CANCELED"
```

### Сценарий 3: Частичный возврат

```bash
# 1. Создать и оплатить заказ на 25000 копеек
ORDER_ID=$(grpcurl -plaintext -d '{
  "customer_id": "customer-789",
  "currency": "USD",
  "items": [{"sku": "SKU-003", "qty": 5, "price": {"currency": "USD", "amount_minor": 5000}}]
}' localhost:50051 oms.v1.OrderService/CreateOrder | jq -r '.order_id')

grpcurl -plaintext -d "{\"order_id\": \"$ORDER_ID\"}" \
  localhost:50051 oms.v1.OrderService/PayOrder

sleep 2

# 2. Вернуть 10000 копеек (2 товара)
grpcurl -plaintext -d "{
  \"order_id\": \"$ORDER_ID\",
  \"amount\": {\"currency\": \"USD\", \"amount_minor\": 10000},
  \"reason\": \"2 items damaged\"
}" localhost:50051 oms.v1.OrderService/RefundOrder

# 3. Проверить результат
grpcurl -plaintext -d "{\"order_id\": \"$ORDER_ID\"}" \
  localhost:50051 oms.v1.OrderService/GetOrder | jq '.order'
```

---

## Обработка ошибок

### Типичные ошибки

#### Order Not Found

```json
{
  "code": "NOT_FOUND",
  "message": "order not found"
}
```

**Решение:** Проверьте правильность `order_id`.

#### Invalid Status

```json
{
  "code": "FAILED_PRECONDITION",
  "message": "order status must be pending"
}
```

**Решение:** Операция недоступна для текущего статуса заказа.

#### Validation Error

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "customer_id is required"
}
```

**Решение:** Проверьте обязательные поля.

#### Inventory Unavailable

```json
{
  "code": "RESOURCE_EXHAUSTED",
  "message": "inventory unavailable"
}
```

**Решение:** Товар отсутствует на складе. Заказ будет автоматически отменён.

#### Payment Declined

```json
{
  "code": "FAILED_PRECONDITION",
  "message": "payment declined"
}
```

**Решение:** Проблема с оплатой. Заказ будет автоматически отменён с освобождением резерва.

---

## Отладка

### Просмотр логов

```bash
# Docker Compose
docker compose logs -f oms

# Локальный запуск
# Логи выводятся в stdout
```

### Проверка метрик

```bash
# Prometheus
open http://localhost:9091

# Grafana
open http://localhost:3000
```

### Kafka события

```bash
# Kafka UI
open http://localhost:8080

# Просмотр топиков
docker exec -it kafka kafka-topics --list --bootstrap-server localhost:9092

# Чтение событий саги
docker exec -it kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic oms.saga.events \
  --from-beginning
```

---

## Тестирование

### Автоматизированные тесты

```bash
# Запустить demo скрипт
make demo-run

# Запустить refund сценарий
make demo-refund

# Нагрузочное тестирование через internal runner (100 сценариев)
make load

# Soak-тест по времени (пример 15 минут)
DURATION=15m make load-soak
```

### Ручное тестирование

```bash
# Список всех методов
grpcurl -plaintext localhost:50051 list

# Описание сервиса
grpcurl -plaintext localhost:50051 describe oms.v1.OrderService

# Описание метода
grpcurl -plaintext localhost:50051 describe oms.v1.OrderService.CreateOrder
```

---

## Мониторинг Saga

### Проверка статуса саги

```bash
# Метрики саги в Prometheus
curl -s http://localhost:9091/metrics | grep oms_saga

# Пример вывода:
# oms_saga_started_total 10
# oms_saga_completed_total 8
# oms_saga_failed_total 2
# oms_saga_canceled_total 0
```

### Timeline события

Timeline показывает все события заказа в хронологическом порядке:

```bash
grpcurl -plaintext -d '{"order_id": "01HQZX..."}' \
  localhost:50051 oms.v1.OrderService/GetOrder | jq '.timeline'
```

Пример timeline для успешного заказа:
```json
[
  {"event_type": "OrderCreated", "timestamp": "..."},
  {"event_type": "OrderStatusChanged", "timestamp": "...", "payload": "{\"status\":\"reserved\"}"},
  {"event_type": "OrderStatusChanged", "timestamp": "...", "payload": "{\"status\":\"paid\"}"},
  {"event_type": "OrderStatusChanged", "timestamp": "...", "payload": "{\"status\":\"confirmed\"}"}
]
```

---

## Дополнительные ресурсы

- [Protobuf определения](../../proto/oms/v1/order_service.proto)
- [Saga документация](../architecture/saga.md)
- [Kafka Integration](./kafka.md)
- [Архитектура](../architecture/overview.md)

---

** Совет:** Используйте `make demo` для быстрого запуска всех сценариев!
