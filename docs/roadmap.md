# Roadmap

Версия: v1.1 • Последнее обновление: 2025-09-27

## TL;DR
- 1: Домен + базовый API, идемпотентность, базовая observability.
- 2: Саги и Outbox, интеграции Inventory/Payment, E2E-тесты.
- 3: Компенсации/Refund, retry/DLQ, нагрузочные тесты.
- 4: mTLS/RBAC, Secret Manager, SLO/alerts, runbooks.
- 5: Prod-ready K8s + CI/CD + Canary.
- 6: REST gateway, schema registry, отчётность, обновление ADR.

## Фаза 1 — Domain & API v1
- Схема БД и миграции.
- gRPC `OrderService`: CreateOrder, GetOrder, ListOrders.
- Идемпотентность CreateOrder.
- Базовая observability: RPC-метрики, JSON-логи, минимальный трейсинг.
- Unit-тесты домена, дымовые интеграционные.

## Фаза 2 — Sagas & Outbox
- Оркестратор: Reserve → Pay → Confirm.
- Интеграции Inventory/Payment (моки/адаптеры).
- Transactional Outbox + publisher воркеры.
- Событие `OrderStatusChanged`.
- E2E-тесты плюс сценарии отказов/компенсаций.

## Фаза 3 — Compensations & Refunds
- Потоки Cancel/Refund.
- Политики retry, DLQ, репроцессинг.
- Дашборды/метрики саг.
- Нагрузочные тесты (Create/Pay/Confirm, Cancel/Refund).

## Фаза 4 — Security & Resilience
- mTLS и базовый RBAC.
- Secret Manager, rate limiting, circuit breaker.
- Уточнённые SLO/alerts, runbooks готовы.

## Фаза 5 — Productionization
- K8s манифесты: probes, HPA, PDB, NetworkPolicy.
- CI/CD: lint, unit, integration, contract, security scan, build, deploy.
- Canary для рискованных релизов, проверки миграций.

## Фаза 6 — Enhancements
- Опциональный gRPC-Gateway (REST).
- Schema Registry для событий.
- Расширенная отчётность/аналитика.
- Обновление ADR, план депрекейтов.
