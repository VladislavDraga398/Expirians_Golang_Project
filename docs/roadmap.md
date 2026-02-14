# Roadmap

> Актуальный план развития OMS

**Версия:** v2.4 | **Обновлено:** 2026-02-14 | **Статус:** Актуально

---

## Текущий статус

Проект находится в стадии **stabilization + Phase 6 execution**:
- базовый функционал заказа, Saga и observability работает;
- инфраструктурные заготовки (K8s/Helm/Kafka/CI) есть;
- следующий фокус: production-hardening runtime-части.

---

## 6 приоритетов (текущий фокус)

### 1. Stabilize baseline (DONE)
- Зафиксировать стабильность тестов и убрать флаки.
- Добиться устойчивого `go test -race ./...`.
- Результат: тесты проходят в race-режиме, включая `integration -count=10`.

### 2. Graceful shutdown end-to-end (DONE)
- Корректно завершать gRPC/HTTP серверы.
- Дождаться завершения фоновых saga-задач перед остановкой процесса.
- Не принимать новые асинхронные saga-dispatch во время shutdown.

### 3. Documentation consolidation (IN PROGRESS)
- Убрать расхождения между docs и runtime.
- Убрать битые ссылки и дубли.
- Держать единый индекс документации как входную точку.

### 4. PostgreSQL migration (DONE)
- Вынести runtime-хранилище из in-memory в PostgreSQL.
- Реализовать `OrderRepository`, `OutboxRepository`, `TimelineRepository`, `IdempotencyRepository`.
- Подключить миграции к процессу запуска/CI.

Текущий прогресс:
- добавлен storage-switch `OMS_STORAGE_DRIVER=memory|postgres`;
- добавлены PostgreSQL-репозитории для `OrderRepository`, `OutboxRepository`, `TimelineRepository`;
- добавлен PostgreSQL-репозиторий `IdempotencyRepository`;
- добавлены versioned SQL миграции `up/down` + CLI `cmd/migrate`;
- добавлена миграция `0002_idempotency_keys` (`up/down`);
- в CI добавлен обязательный `migration_check` gate (`up -> down -> up`) + PostgreSQL integration tests для idempotency-репозитория;
- добавлена auto-migration при старте (`OMS_POSTGRES_AUTO_MIGRATE=true`);
- health-check теперь умеет проверять доступность PostgreSQL.

### 5. Outbox publisher worker (IN PROGRESS)
- Реализовать публикацию из outbox как отдельный воркер:
  - `pending -> sent/failed`
  - retry policy
  - интеграция с DLQ
- Убрать разрыв между `enqueue` и реальной доставкой событий.

Текущий прогресс:
- расширен контракт `OutboxRepository` для `pull pending` + `mark sent/failed`;
- реализован runtime worker публикации outbox-сообщений;
- добавлен Kafka outbox publisher + fallback публикация в DLQ;
- worker подключён к lifecycle приложения (start/stop вместе с сервисом).

### 6. Idempotency enforcement (IN PROGRESS)
- Включить обязательный `idempotency-key` для мутаций.
- Хранить request hash + response cache + статус обработки.
- Гарантировать безопасный replay без двойного эффекта.

Текущий прогресс:
- обязательный `idempotency-key` включён для mutating gRPC RPC (`CreateOrder`, `PayOrder`, `CancelOrder`, `RefundOrder`);
- включено кэширование успешного ответа и replay для повторов с тем же `request_hash`;
- конфликт `idempotency-key` с другим `request_hash` возвращает `AlreadyExists`.

---

## Что уже сделано по стабилизации (2026-02-12)

- Добавлен управляемый graceful shutdown для фоновых saga-задач.
- Добавлен `grpc health` registration.
- Добавлен endpoint `readyz` в HTTP health surface.
- Устранён флак интеграционных тестов по timeline-событиям.
- Обновлены ключевые документы и ссылки.

---

## Следующий шаг

Завершить hardening **Outbox + Idempotency**: добавить операционные алерты/регламент репроцессинга DLQ, TTL-cleanup job для `idempotency_keys` и формализовать политику retention.

Параллельный техдолг по качеству кода: убрать скрытые fallback'и в конфиге, вынести магические значения в константы и поддерживать единый вход тестовых прогонов через `test/run/*`.
