# Idempotency

> Обеспечение идемпотентности операций

**Версия:** v2.4 | **Обновлено:** 2026-02-23 | **Статус:** Implemented in core

---

## TL;DR
- Текущая модель: все мутации требуют `Idempotency-Key`; повтор отдаёт тот же ответ/код.
- Хранение: таблица `idempotency_keys`; runtime TTL фиксирован `24h`.
- Конфликт ключа с другим `request_hash` → `AlreadyExists`/`InvalidArgument`.
- Идемпотентность событий — через inbox/`processed_events` на потребителе.
- Текущий runtime-статус: storage-layer (memory + postgres) реализован, обязательная проверка `idempotency-key` включена для mutating gRPC RPC (`CreateOrder`, `PayOrder`, `CancelOrder`, `RefundOrder`).

## Политика и отпечаток
- `request_hash = hash(method + route/rpc + canonicalized body + critical headers)`.
- Ключ обязателен; повтор с иным `request_hash` запрещён.

## Хранилище `idempotency_keys`
- Поля: `key`, `request_hash`, `response_body`, `status (processing|done|failed)`, `http_status/grpc_code`, `ttl_at`, timestamps.
- Индексы: PK(`key`), `ttl_at`, `status`; Redis/кэш по желанию.

## Жизненный цикл
1. INSERT `processing` + `request_hash`.
2. Выполнить бизнес-операцию.
3. При успехе: UPDATE → `done`, сохранить ответ/код.
4. При ошибке: UPDATE → `failed`, сохранить детали.
5. Повтор: `processing` → 425/409; `done` → сохранить ответ; `failed` → вернуть ошибку.

## TTL и очистка
- Runtime TTL для idempotency record: `24h`.
- Cleanup worker удаляет просроченные записи по конфигу (`OMS_IDEMPOTENCY_CLEANUP_INTERVAL`, `OMS_IDEMPOTENCY_CLEANUP_BATCH_SIZE`).
- После TTL ключ считается новым.

## gRPC и события
- Передача ключа через metadata `idempotency-key`.
- Потребители событий ведут `processed_events` для дедупликации.

## Метрики/алерты
- `oms_idempotency_cleanup_runs_total{result}`, `oms_idempotency_cleanup_deleted_total`, `oms_idempotency_cleanup_last_deleted`.
- Дополнительно для конфликтов отслеживаются gRPC коды `AlreadyExists`/`Aborted` на mutating RPC.

## Альтернативы
- Хранить только статус (без ответа) — проще, но нельзя переиспользовать response.
- Укороченный TTL — меньше таблица, но повторы станут новой операцией.
