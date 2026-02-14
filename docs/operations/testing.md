# Testing Strategy

> Стратегия тестирования OMS

**Версия:** v2.2 | **Обновлено:** 2026-02-12 | **Статус:** Актуально

---

## TL;DR
- Пирамида: Unit → Integration → Contract → Load/Chaos.
- DoD: покрытие домена ≥80%, E2E охватывают happy/fail/compensation/unknown, контракты зелёные, нагрузочные в SLO.
- Для PR merge обязателен `premerge_stand` (integration stand + observability gate + load gate) в CI.
- Для PR merge обязателен `migration_check` (up/down/up SQL миграций) в CI.
- Для PR в `dev` используется dev-профиль стенда; для PR в `main`/`master` используется release-профиль.

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
- Pipeline: Lint → Tests → Migration Check → Build → Pre-Merge Stand (PR) → Security/Docker → Summary.
- Локальная единая точка входа для запуска тестов: `test/run/*` (`all.sh`, `unit.sh`, `integration.sh`, `race.sh`).
- `premerge_stand` поднимает стенд через Docker Compose (`oms`, `zookeeper`, `kafka`, `prometheus`).
- `migration_check` проверяет SQL-модель через последовательность `up -> down(1) -> up` и запускает PostgreSQL integration tests для idempotency storage-path.
- Этапы `premerge_stand`: lifecycle smoke (`scripts/saga_load.sh`), observability gate (`scripts/ci/observability_gate.sh`), load gate (`scripts/ci/load_gate.sh`, запускает `cmd/loadtest`).
- Профиль стенда выбирается по целевой ветке PR (`github.base_ref`): `dev` использует быстрый gate для ежедневной разработки, `main`/`master` использует более строгий release gate.
- Артефакты: coverage, логи стенда на фейлах, контейнерные образы.
