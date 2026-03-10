# API Specification

> Спецификация gRPC API для OMS

**Версия:** v2.3 | **Обновлено:** 2026-03-08 | **Статус:** Sprint 3 Active

---

## TL;DR
- Публичные gRPC-контракты runtime: `OrderService` и `CourierService`.
- Для mutating RPC (`CreateOrder`, `PayOrder`, `CancelOrder`, `RefundOrder`) `idempotency-key` обязателен.
- Для mutating RPC `CourierService` `idempotency-key` пока не требуется.
- Ошибки: gRPC codes + details; `AlreadyExists` при конфликте ключа идемпотентности.
- REST-gateway маппинг описан в proto, но в текущем runtime gateway не поднят.

## Назначение
Публичные gRPC-контракты (`OrderService`, `CourierService`) и правила обработки ошибок/идемпотентности.

## Версионирование и пакет
- Версия API: v1
- Пакет: `oms.v1`

## Метаданные
- `idempotency-key` обязателен для mutating RPC (`CreateOrder`, `PayOrder`, `CancelOrder`, `RefundOrder`).
- Для `GetOrder`/`ListOrders` `idempotency-key` не требуется.
- `x-correlation-id` как обязательный runtime-контракт пока не введён (может использоваться внешним слоем).

## Модель ошибок
- Используем gRPC status codes с расширенными деталями.
- Частые коды:
  - InvalidArgument — ошибки валидации.
  - AlreadyExists — ключ идемпотентности переиспользован с другим payload.
  - NotFound — заказ не найден.
  - FailedPrecondition — некорректный переход состояния.
  - Aborted — конфликт optimistic locking или запрос с тем же `idempotency-key` уже находится в `processing`.
  - DeadlineExceeded/Unavailable — проблемы зависимостей/временные сбои.

## OrderService (публичный)
- Методы
  - `CreateOrder(CreateOrderRequest) returns (CreateOrderResponse)`
  - `GetOrder(GetOrderRequest) returns (GetOrderResponse)`
  - `ListOrders(ListOrdersRequest) returns (ListOrdersResponse)`
  - `PayOrder(PayOrderRequest) returns (PayOrderResponse)`
  - `CancelOrder(CancelOrderRequest) returns (CancelOrderResponse)`
  - `RefundOrder(RefundOrderRequest) returns (RefundOrderResponse)`

## CourierService (публичный)
- Методы
  - `RegisterCourier(RegisterCourierRequest) returns (RegisterCourierResponse)`
  - `GetCourier(GetCourierRequest) returns (GetCourierResponse)`
  - `ListCouriersByZone(ListCouriersByZoneRequest) returns (ListCouriersByZoneResponse)`
  - `ReplaceCourierZones(ReplaceCourierZonesRequest) returns (ReplaceCourierZonesResponse)`
  - `CreateCourierSlot(CreateCourierSlotRequest) returns (CreateCourierSlotResponse)`
  - `ListCourierSlots(ListCourierSlotsRequest) returns (ListCourierSlotsResponse)`
  - `GetCourierVehicleCapability(GetCourierVehicleCapabilityRequest) returns (GetCourierVehicleCapabilityResponse)`
  - `ListCourierVehicleCapabilities(ListCourierVehicleCapabilitiesRequest) returns (ListCourierVehicleCapabilitiesResponse)`
  - `SubmitCourierRating(SubmitCourierRatingRequest) returns (SubmitCourierRatingResponse)`
  - `GetCourierRatingSummary(GetCourierRatingSummaryRequest) returns (GetCourierRatingSummaryResponse)`

## CourierService — ключевые доменные правила runtime
- Регистрация курьера:
  - телефон обязателен и уникален;
  - для `scooter|bike` разрешена одна зона;
  - для `car` допускаются несколько зон;
  - должен быть ровно один primary zone (если не задан, проставляется первой зоне).
- Слоты:
  - поддерживаемые длительности `4|8|12` часов;
  - ночной слот `20:00-08:00` разрешён только для `car`;
  - конфликтующие слоты одного курьера отклоняются.
- Рейтинг:
  - `score` в диапазоне `1..5`;
  - при `score < 3` обязательно передать минимум один негативный reason-tag;
  - при `score = 5` разрешены только позитивные теги.

## Сообщения (фрагмент)
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
  repeated OrderItem items = 5;
  int64 version = 6;
  string currency = 7;
}
```

## События (асинхронные контракты)
- `OrderStatusChanged { order_id, prev_status, new_status, reason, seq, occurred_at, schema_version }`
- `PaymentStatusChanged { order_id, payment_id, prev_status, new_status, provider, external_id, seq, occurred_at, schema_version }`
- Ключ дедупликации: `(order_id, event_type, seq)`.

## Пагинация
- Текущий runtime `ListOrders` использует `customer_id` + `page_size` (limit).
- `page_token` и `filter_statuses` зарезервированы в proto, но пока не задействованы в runtime.
- `next_page_token` пока возвращается пустым.

## REST Gateway (опционально)
- Примеры мэппинга:
  - POST `/v1/orders` → `CreateOrder`
  - GET `/v1/orders/{order_id}` → `GetOrder`
  - GET `/v1/orders` → `ListOrders`
  - POST `/v1/orders/{order_id}/pay` → `PayOrder`
  - POST `/v1/orders/{order_id}/cancel` → `CancelOrder`
  - POST `/v1/orders/{order_id}/refund` → `RefundOrder`
  - POST `/v1/couriers` → `RegisterCourier`
  - GET `/v1/couriers/{courier_id}` → `GetCourier`
  - GET `/v1/zones/{zone_id}/couriers` → `ListCouriersByZone`
  - PUT `/v1/couriers/{courier_id}/zones` → `ReplaceCourierZones`
  - POST `/v1/couriers/{courier_id}/slots` → `CreateCourierSlot`
  - GET `/v1/couriers/{courier_id}/slots` → `ListCourierSlots`
  - GET `/v1/courier-vehicle-capabilities/{vehicle_type}` → `GetCourierVehicleCapability`
  - GET `/v1/courier-vehicle-capabilities` → `ListCourierVehicleCapabilities`
  - POST `/v1/couriers/{courier_id}/ratings` → `SubmitCourierRating`
  - GET `/v1/couriers/{courier_id}/ratings/summary` → `GetCourierRatingSummary`

## Поведение идемпотентности
- Runtime-поведение: один и тот же ключ + тот же payload → повторно возвращается сохранённый ответ.
- Конфликт ключа с иным `request_hash` → `AlreadyExists`.
- Повтор с ключом в статусе `processing` → `Aborted`.
