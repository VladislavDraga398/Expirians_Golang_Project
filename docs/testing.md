# Testing Strategy

Версия: v1.1 • Последнее обновление: 2025-09-27

## TL;DR
- Пирамида: Unit → Integration → Contract → Load/Chaos.
- DoD: покрытие домена ≥80%, E2E охватывают happy/fail/compensation/unknown, контракты зелёные, нагрузочные в SLO.
- CI pipeline: Lint → Unit → Integration (контейнеры) → Contract → Build → Security Scan → Publish.

## Цели
- Проверить доменную логику, устойчивость саг/компенсаций, стабильность контрактов и перфоманс.

## Пирамида и сценарии
- **Unit:** агрегаты, инварианты, `request_hash`, backoff+jitter, dry-run оркестратора.
- **Integration:** контейнеры БД+брокера, сценарии Create→Confirm, Reserve fail, Pay fail, Cancel/Refund, повторные `Idempotency-Key`, Outbox+DLQ.
- **Contract:** gRPC позитив/негатив, события (`schema_version`, дедуп-ключ).
- **Load/Chaos:** спайки, плавный рост 100–500 RPS, смешанные потоки, fault injection (disconnect, таймауты, дедлоки).

## Данные и фикстуры
- Реалистичные SKU/цены (minor units), валюты.
- Детерминированные сиды и время; cleanup после прогонов.

## Критерии приёмки (DoD)
- Unit-покрытие критических путей ≥ 80%.
- Интеграционные E2E закрывают happy/fail/compensation/unknown.
- Контрактные тесты зелёные для всех RPC/событий.
- Нагрузочные тесты: p95/p99 в SLO, outbox/DLQ без роста.

## Автоматизация в CI
- Pipeline: Lint → Unit → Integration (контейнеры) → Contract → Build → Security Scan → Publish.
- Артефакты: coverage, логи/трейсы на фейлах, контейнерные образы.

