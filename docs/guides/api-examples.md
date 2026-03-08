# API Examples

Практические примеры вызова gRPC API OMS через `grpcurl`.

**Версия:** v2.3 | **Обновлено:** 2026-03-08 | **Статус:** Sprint 3 Active

---

## Важно перед стартом

- Mutating RPC требуют metadata `idempotency-key`:
  - `CreateOrder`
  - `PayOrder`
  - `CancelOrder`
  - `RefundOrder`
- Сага для `PayOrder`/`CancelOrder`/`RefundOrder` выполняется асинхронно, статус проверяется через `GetOrder`.

---

## CreateOrder

```bash
grpcurl -plaintext \
  -H 'idempotency-key: create-order-001' \
  -d '{
    "customer_id": "customer-123",
    "currency": "RUB",
    "items": [
      {
        "sku": "CREATINE-300G",
        "qty": 1,
        "price": {"currency": "RUB", "amount_minor": 199900}
      },
      {
        "sku": "WHEY-900G",
        "qty": 2,
        "price": {"currency": "RUB", "amount_minor": 249900}
      }
    ]
  }' \
  localhost:50051 oms.v1.OrderService/CreateOrder
```

Пример ответа:

```json
{
  "order": {
    "id": "a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
    "customer_id": "customer-123",
    "status": "ORDER_STATUS_PENDING",
    "amount": {
      "currency": "RUB",
      "amount_minor": "699700"
    },
    "items": [
      {
        "sku": "CREATINE-300G",
        "qty": 1,
        "price": {"currency": "RUB", "amount_minor": "199900"}
      }
    ],
    "version": "0",
    "currency": "RUB"
  }
}
```

---

## GetOrder

```bash
grpcurl -plaintext \
  -d '{"order_id":"a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c"}' \
  localhost:50051 oms.v1.OrderService/GetOrder
```

Пример ответа:

```json
{
  "order": {
    "id": "a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
    "customer_id": "customer-123",
    "status": "ORDER_STATUS_PENDING",
    "amount": {
      "currency": "RUB",
      "amount_minor": "699700"
    },
    "items": [
      {
        "sku": "CREATINE-300G",
        "qty": 1,
        "price": {"currency": "RUB", "amount_minor": "199900"}
      }
    ],
    "version": "0",
    "currency": "RUB"
  },
  "timeline": [
    {
      "type": "OrderStatusChanged",
      "reason": "pending",
      "unix_time": "1740266400"
    }
  ]
}
```

---

## PayOrder

```bash
grpcurl -plaintext \
  -H 'idempotency-key: pay-order-001' \
  -d '{"order_id":"a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c"}' \
  localhost:50051 oms.v1.OrderService/PayOrder
```

Пример ответа:

```json
{
  "order_id": "a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
  "status": "ORDER_STATUS_PENDING"
}
```

---

## CancelOrder

```bash
grpcurl -plaintext \
  -H 'idempotency-key: cancel-order-001' \
  -d '{
    "order_id":"a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
    "reason":"customer_request"
  }' \
  localhost:50051 oms.v1.OrderService/CancelOrder
```

Пример ответа:

```json
{
  "order_id": "a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
  "status": "ORDER_STATUS_CANCELED"
}
```

---

## RefundOrder

```bash
grpcurl -plaintext \
  -H 'idempotency-key: refund-order-001' \
  -d '{
    "order_id":"a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
    "amount":{"currency":"RUB","amount_minor":100000},
    "reason":"quality_issue"
  }' \
  localhost:50051 oms.v1.OrderService/RefundOrder
```

Пример ответа:

```json
{
  "order_id": "a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
  "status": "ORDER_STATUS_REFUNDED"
}
```

---

## ListOrders

```bash
grpcurl -plaintext \
  -d '{"customer_id":"customer-123","page_size":20}' \
  localhost:50051 oms.v1.OrderService/ListOrders
```

Пример ответа:

```json
{
  "orders": [
    {
      "id": "a33e8f8a-3dbe-4f44-8e39-8f6e2de95f7c",
      "customer_id": "customer-123",
      "status": "ORDER_STATUS_CONFIRMED",
      "amount": {"currency": "RUB", "amount_minor": "699700"},
      "items": [],
      "version": "3",
      "currency": "RUB"
    }
  ],
  "next_page_token": ""
}
```

---

## RegisterCourier

```bash
grpcurl -plaintext \
  -d '{
    "phone": "+79991234567",
    "first_name": "Ivan",
    "last_name": "Petrov",
    "vehicle_type": "COURIER_VEHICLE_TYPE_CAR",
    "zones": [
      {"zone_id":"ЦАО","is_primary":true},
      {"zone_id":"САО","is_primary":false}
    ]
  }' \
  localhost:50051 oms.v1.CourierService/RegisterCourier
```

---

## CreateCourierSlot (night shift for car)

```bash
grpcurl -plaintext \
  -d '{
    "courier_id":"courier-123",
    "slot_start_unix": 1761930000,
    "slot_end_unix": 1761973200,
    "duration_hours": 12
  }' \
  localhost:50051 oms.v1.CourierService/CreateCourierSlot
```

---

## ListCourierVehicleCapabilities

```bash
grpcurl -plaintext \
  -d '{}' \
  localhost:50051 oms.v1.CourierService/ListCourierVehicleCapabilities
```

---

## SubmitCourierRating

```bash
grpcurl -plaintext \
  -d '{
    "courier_id":"courier-123",
    "score": 2,
    "tags": ["COURIER_RATING_TAG_DELAYED_DELIVERY"],
    "comment": "Опоздание на 25 минут"
  }' \
  localhost:50051 oms.v1.CourierService/SubmitCourierRating
```

---

## GetCourierRatingSummary

```bash
grpcurl -plaintext \
  -d '{"courier_id":"courier-123"}' \
  localhost:50051 oms.v1.CourierService/GetCourierRatingSummary
```

---

## Частые ошибки

- `InvalidArgument`: невалидный payload или отсутствует обязательный `idempotency-key` для mutating RPC.
- `AlreadyExists`: повторное использование `idempotency-key` с другим payload.
- `Aborted`: запрос с тем же `idempotency-key` уже в статусе `processing`.
- `FailedPrecondition`: операция не разрешена для текущего статуса заказа.
