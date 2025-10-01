# 📡 API Specification

> Спецификация gRPC API для OMS

**Версия:** v2.0 | **Обновлено:** 2025-10-01 | **Статус:** Актуально

---

## 🎯 TL;DR
- Публичный `OrderService` + внутренние `InventoryService`/`PaymentService`.
- Метаданные: `idempotency-key`, `x-correlation-id`.
- Ошибки: gRPC codes + details; `AlreadyExists` при конфликте ключа идемпотентности.
- Пагинация: keyset через `page_token`.

## Назначение
Публичные и внутренние gRPC-контракты (orders, inventory, payment) и рекомендации по ошибкам, идемпотентности, пагинации и опциональному REST-gateway.

## Версионирование и пакет
- Версия API: v1
- Пакет: `oms.v1`

## Метаданные
- Идемпотентность: `idempotency-key: <string>` (gRPC metadata) для всех мутаций.
- Корреляция: `x-correlation-id` (генерируется, если отсутствует).

## Модель ошибок
- Используем gRPC status codes с расширенными деталями.
- Частые коды:
  - InvalidArgument — ошибки валидации.
  - AlreadyExists — ключ идемпотентности переиспользован с другим payload.
  - NotFound — заказ не найден.
  - FailedPrecondition — некорректный переход состояния.
  - Aborted — конфликт optimistic locking.
  - DeadlineExceeded/Unavailable — проблемы зависимостей/временные сбои.

## OrderService (публичный)
- Методы
  - `CreateOrder(CreateOrderRequest) returns (CreateOrderResponse)`
  - `GetOrder(GetOrderRequest) returns (GetOrderResponse)`
  - `ListOrders(ListOrdersRequest) returns (ListOrdersResponse)`
  - `PayOrder(PayOrderRequest) returns (PayOrderResponse)`
  - `CancelOrder(CancelOrderRequest) returns (CancelOrderResponse)`
  - `RefundOrder(RefundOrderRequest) returns (RefundOrderResponse)`

-- Сообщения (фрагмент)
```proto
message Money { string currency = 1; int64 amount_minor = 2; }
message OrderItem { string sku = 1; int32 qty = 2; Money price = 3; }

enum OrderStatus {
  ORDER_STATUS_UNSPECIFIED = 0;
  ORDER_STATUS_PENDING = 1;
  ORDER_STATUS_RESERVED = 2;
  ORDER_STATUS_PAID = 3;
  ORDER_STATUS_CONFIRMED = 4;
  ORDER_STATUS_CANCELED = 5;
  ORDER_STATUS_REFUNDED = 6;
}

message Order {
  string id = 1;
  string customer_id = 2;
  OrderStatus status = 3;
  Money amount = 4;
  repeated OrderItem items = 6;
  int64 version = 7;
}
```

## InventoryService (внутренний)
- `Reserve(ReserveRequest) returns (ReserveResponse)`
- `Release(ReleaseRequest) returns (ReleaseResponse)`

## PaymentService (внутренний)
- `Hold(HoldRequest) returns (HoldResponse)`
- `Capture(CaptureRequest) returns (CaptureResponse)`
- `Refund(RefundRequest) returns (RefundResponse)`

## События (асинхронные контракты)
- `OrderStatusChanged { order_id, prev_status, new_status, reason, seq, occurred_at, schema_version }`
- `PaymentStatusChanged { order_id, payment_id, prev_status, new_status, provider, external_id, seq, occurred_at, schema_version }`
- Ключ дедупликации: `(order_id, event_type, seq)`.

## Пагинация
- `ListOrders`: `page_size` 1..100 (по умолчанию 20), `page_token` (opaque keyset).

## REST Gateway (опционально)
- Примеры мэппинга:
  - POST `/v1/orders` → `CreateOrder`
  - GET `/v1/orders/{id}` → `GetOrder`
  - GET `/v1/orders` → `ListOrders`
  - POST `/v1/orders/{id}:pay` → `PayOrder`
  - POST `/v1/orders/{id}:cancel` → `CancelOrder`
  - POST `/v1/orders/{id}:refund` → `RefundOrder`

## Поведение идемпотентности
- Один и тот же ключ → идентичный ответ/код.
- Конфликт ключа с иным `request_hash` → `AlreadyExists`/`InvalidArgument` с деталями.

