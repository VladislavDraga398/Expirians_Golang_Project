# Observability

Версия: v1.1 • Последнее обновление: 2025-09-27

## TL;DR
- SLI/SLO: доступность 99.9%, `CreateOrder` p95 ≤ 300 мс, E2E ≤ 3 мин, доставки Outbox ≥ 99.5% без DLQ.
- Метрики: RED+USE для gRPC, саги, outbox, идемпотентность.
- Логи: структурированный JSON с `trace_id`, `order_id`, `saga_step`.
- Трейсинг: входной RPC → шаги саги → вызовы зависимостей → публикация.
- Алерты: error rate, p95, outbox backlog, DLQ, зависимость/идемпотентность.

## Назначение
Определить метрики, логи, трейсинг, SLI/SLO и алерты для OMS.

## Метрики (примеры)
- gRPC server
  - `rpc_server_requests_total{method,code}`
  - `rpc_server_latency_seconds{method}` (histограмма)
  - `rpc_server_inflight_requests{method}`
- gRPC client (зависимости)
  - `rpc_client_requests_total{dep}`
  - `rpc_client_latency_seconds{dep}`
  - состояние circuit breaker, количество ретраев
- Саги/бизнес
  - `saga_step_duration_seconds{step}` (hist)
  - `saga_flow_transitions_total{from,to}`
  - `order_state_total{status}`
  - `order_e2e_latency_seconds` (hist)
- Outbox
  - `outbox_pending_records`
  - `outbox_oldest_pending_age_seconds`
  - `outbox_publish_attempts_total{result}`
  - `outbox_dlq_total`
- Идемпотентность
  - `idempotency_conflicts_total`
  - `idempotency_processing_gauge`

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

## Health/Readiness
- Health включает проверки зависимостей (БД, брокер, бэклог publisher).
- Readiness зависит от критичных зависимостей и допустимого бэклога.

