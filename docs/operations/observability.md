# Observability

> Мониторинг, логирование и трейсинг

**Версия:** v2.2 | **Обновлено:** 2026-02-20 | **Статус:** Актуально

---

## TL;DR
- SLI/SLO: доступность 99.9%, `CreateOrder` p95 ≤ 300 мс, E2E ≤ 3 мин, доставки Outbox ≥ 99.5% без DLQ.
- Метрики: gRPC + бизнес-метрики саг и outbox.
- Логи: структурированный JSON с `trace_id`, `order_id`, `saga_step`.
- Трейсинг: входной RPC → шаги саги → вызовы зависимостей → публикация.
- PR merge gate включает observability проверку: health/readiness, `/metrics`, наличие и рост ключевых метрик, проверка scrape в Prometheus.

## Назначение
Определить метрики, логи, трейсинг, SLI/SLO и алерты для OMS.

## Метрики (текущая реализация)
- gRPC server (grpc-prometheus): `grpc_server_started_total`, `grpc_server_handled_total`, `grpc_server_handling_seconds_*`.
- Saga/бизнес: `oms_saga_started_total`, `oms_saga_completed_total`, `oms_saga_canceled_total`, `oms_saga_refunded_total`, `oms_saga_failed_total`, `oms_saga_duration_seconds_*`, `oms_saga_step_duration_seconds_*`, `oms_active_sagas`.
- Timeline/Outbox: `oms_timeline_events_total`, `oms_outbox_events_total`.
- Outbox backlog/runtime: `oms_outbox_publish_attempts_total{result}`, `oms_outbox_pending_records`, `oms_outbox_oldest_pending_age_seconds`.
- Idempotency cleanup: `oms_idempotency_cleanup_runs_total{result}`, `oms_idempotency_cleanup_deleted_total`, `oms_idempotency_cleanup_last_deleted`.
- Runtime: `go_*`, `process_*`.

## CI Observability Gate
Скрипт: `scripts/ci/observability_gate.sh`

Проверяет:
- HTTP endpoints: `/healthz`, `/livez`, `/readyz` (HTTP 200).
- Доступность `/metrics`.
- Наличие ключевых серий метрик (`oms_*`, runtime).
- Рост счетчиков после smoke-нагрузки: `oms_saga_started_total > 0`, терминальные saga-счетчики (`completed + canceled + failed`) > 0, `oms_saga_duration_seconds_count > 0`, `oms_timeline_events_total > 0`.
- Аномалия `oms_active_sagas < 0` помечается как warning и должна разбираться отдельно.
- При `CHECK_PROMETHEUS=1`: scrape-path в Prometheus (`up{job="oms"} == 1`).

## Логи
- Поля JSON: `ts`, `level`, `logger`, `msg`, `trace_id`, `span_id`, `correlation_id`, `order_id`, `saga_step`, `status`, `duration_ms`, `err_code`, `err_detail`.
- Политика: Info для смены статусов, Debug для деталей шагов (по флагу), Error для техошибок.

## Трейсинг
- Спаны: входной RPC, запись в БД + outbox, шаги саги, внешние вызовы, публикация.
- Атрибуты: `order.id`, `saga.step`, `retry.attempt`, `dep.name`.
- Семплинг: базово 1–5%, tail-sampling для ошибок/медленных запросов.

## Дашборды (идеи)
- API Overview: RPS, error rate, p95/p99 по методам.
- Sagas: воронка переходов, доля cancel/refund, p95 шагов.
- Outbox: pending, возраст старейшей записи, попытки, приток в DLQ.
- Dependencies: клиентская латентность, доля открытого CB, ретраи.
- Idempotency: конфликты, «долго processing» ключи.

## Алерты (примерные пороги)
- Ошибки API > 2% (5 мин), CreateOrder p95 > 600 мс (10 мин).
- E2E: незавершённых саг > 5% за 10 мин.
- Outbox: возраст старейшего pending > 10 мин; DLQ inflow > 5/мин (окно 5 мин).
- Dependencies: CB открыт > 20% времени (10 мин); client error rate > 5% (5 мин).
- Idempotency: processing-ключи > порога дольше 2 мин.
- Локальный набор alert rules: `deploy/prometheus/alerts.yml`.

## Health/Readiness
- Health включает проверки зависимостей (БД, брокер, бэклог publisher).
- Readiness зависит от критичных зависимостей и допустимого бэклога.
