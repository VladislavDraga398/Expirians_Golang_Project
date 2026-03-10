# Data Model

> Актуальная модель данных OMS/BoostMarket на текущем runtime

**Версия:** v2.4 | **Обновлено:** 2026-03-08 | **Статус:** Sprint 3 Active

---

## TL;DR
- Основные таблицы заказа: `orders`, `order_items`, `timeline_events`, `outbox_messages`, `idempotency_keys`.
- Денежные суммы: minor units (`BIGINT`).
- Outbox runtime использует статусы `pending|processing|sent|failed` и поле `attempt_count`.
- Delivery-модель расширена: `couriers`, `courier_zones`, `courier_slots`, `courier_vehicle_capabilities`, `courier_ratings`.
- `CourierService` уже выведен в runtime (registration/zones/slots/capabilities/ratings).

## Основные сущности (ядро OMS)

### `orders`
- `id` (PK, text)
- `customer_id` (text)
- `status` (text)
- `currency` (text)
- `amount_minor` (bigint)
- `version` (bigint)
- `created_at`, `updated_at` (timestamptz)

Индексы:
- `idx_orders_customer_created_at (customer_id, created_at DESC)`

### `order_items`
- `id` (PK)
- `order_id` (FK -> `orders.id`)
- `sku`, `qty`, `price_minor`
- `created_at`

Индексы:
- `idx_order_items_order_id (order_id)`

### `timeline_events`
- `id` (bigserial PK)
- `order_id` (FK -> `orders.id`)
- `type`, `reason`
- `occurred`

Индексы:
- `idx_timeline_order_occurred (order_id, occurred, id)`

### `outbox_messages`
- `id` (PK)
- `aggregate_type`, `aggregate_id`, `event_type`
- `payload` (bytea)
- `status` (`pending|processing|sent|failed`)
- `attempt_count` (integer)
- `created_at`, `updated_at`

Индексы:
- `idx_outbox_status_created_at (status, created_at)`

### `idempotency_keys`
- `key` (PK)
- `request_hash`
- `response_body` (bytea)
- `http_status` (integer)
- `status` (`processing|done|failed`)
- `ttl_at`, `created_at`, `updated_at`

Индексы:
- `idx_idempotency_keys_ttl_at (ttl_at)`
- `idx_idempotency_keys_status (status)`

## Delivery foundation (Sprint 2 + early Sprint 5)

### `couriers`
- `id` (PK)
- `phone` (unique)
- `first_name`, `last_name`
- `vehicle_type` (`scooter|bike|car`)
- `is_active`
- `created_at`, `updated_at`

Индексы:
- `idx_couriers_vehicle_type_active (vehicle_type, is_active)`

### `courier_zones`
- `courier_id` (FK -> `couriers.id`)
- `zone_id`
- `is_primary`
- `created_at`
- PK: `(courier_id, zone_id)`

Индексы/ограничения:
- `idx_courier_zones_zone_id (zone_id)`
- `idx_courier_zones_courier_priority (courier_id, is_primary DESC, zone_id ASC)`
- `idx_courier_zones_one_primary` (частичный unique: один primary zone на курьера)

### `courier_slots`
- `id` (PK)
- `courier_id` (FK -> `couriers.id`)
- `slot_start`, `slot_end`
- `duration_hours` (`4|8|12`)
- `status` (`planned|active|completed|canceled`)
- `created_at`, `updated_at`

Ограничения:
- `slot_end > slot_start`
- unique `(courier_id, slot_start)`

### `courier_vehicle_capabilities`
- `vehicle_type` (PK: `scooter|bike|car`)
- `max_weight_grams`
- `max_volume_cm3`
- `max_orders_per_trip`
- `updated_at`

### `courier_ratings`
- `id` (PK)
- `courier_id` (FK -> `couriers.id`, `ON DELETE CASCADE`)
- `score` (`1..5`)
- `tags` (JSONB array)
- `comment`
- `created_at`

Индексы:
- `idx_courier_ratings_courier_created (courier_id, created_at DESC)`
- `idx_courier_ratings_courier_score (courier_id, score)`

## Статусы заказа
- `pending`
- `reserved`
- `paid`
- `confirmed`
- `canceled`
- `refunded`

## Текущее состояние runtime
- Публичный `CourierService` включён в runtime.
- Реализованы: регистрация курьеров, управление зонами, слоты, vehicle capabilities, рейтинг и summary.

## Связанные документы
- `docs/architecture/overview.md`
- `docs/architecture/saga.md`
- `docs/architecture/outbox.md`
- `docs/roadmap.md`
