# Idempotency

Версия: v1.1 • Последнее обновление: 2025-09-27

## TL;DR
- Все мутации требуют `Idempotency-Key`; повтор отдаёт тот же ответ/код.
- Хранение: таблица `idempotency_keys` + опциональный кэш; TTL 24–72 ч.
- Конфликт ключа с другим `request_hash` → `AlreadyExists`/`InvalidArgument`.
- Идемпотентность событий — через inbox/`processed_events` на потребителе.

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
- TTL 24–72 ч (конфиг); cron удаляет просроченные записи.
- После TTL ключ считается новым.

## gRPC и события
- Передача ключа через metadata `idempotency-key`.
- Потребители событий ведут `processed_events` для дедупликации.

## Метрики/алерты
- `idempotency_conflicts_total`, `idempotency_processing_gauge`, `idempotency_retries_total`.
- Алерт при росте конфликтов или «зависших» `processing`.

## Альтернативы
- Хранить только статус (без ответа) — проще, но нельзя переиспользовать response.
- Укороченный TTL — меньше таблица, но повторы станут новой операцией.

