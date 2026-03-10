# CI/CD Pipeline

Автоматизированный pipeline для проверки, тестирования и merge-gate проекта OMS.

**Версия:** v2.6 | **Обновлено:** 2026-03-10

---

## TL;DR
- Тестовый стенд не хранится в отдельной ветке: он поднимается как временное окружение в GitHub Actions.
- `premerge_stand` запускается на каждом PR, но с разными профилями по целевой ветке.
- `premerge_stand` всегда запускается в `postgres`-режиме (`OMS_STORAGE_DRIVER=postgres`) для prod-like проверки.
- В pre-merge добавлены `grpc` readiness probe и риск-гейты (`outbox`, `load`, `observability`) с staged rollout (`shadow -> strict`).
- Merge разрешён только после зелёных `lint`, `test`, `build`, `premerge_stand`.
- `test` job включает обязательный coverage gate `>= 80%`.
- SQL-модель дополнительно защищена обязательным `migration_check`.
- Артефакты pre-merge включают `compose` логи, `load-gate-report.json`, snapshot метрик и лог outbox-gate.
- Ночной workflow `Nightly Stand Reliability` в фазе N1 запускается вручную (`workflow_dispatch`), затем переводится в schedule после стабилизации.

---

## Веточная модель и стенд

| Поток | Целевая ветка PR | Профиль стенда | Что проверяем |
|---|---|---|---|
| `feature/* -> dev` | `dev` | `dev` | быстрый smoke + observability gate + облегчённый load gate |
| `dev -> main` / `dev -> master` | `main` / `master` | `release` | расширенный smoke + observability gate + строгий load gate |

Важно: стенд запускается **до merge**, а не после merge.

---

## Когда запускается pipeline

- `push` в `main`, `master`, `dev`
- `pull_request` в `main`, `master`, `dev`

---

## Jobs

### `lint`
- `gofmt`
- `go vet`
- `golangci-lint`

### `test`
- `go test ./... -race -count=1 -v`
- coverage report + coverage gate (`COVERAGE_MIN_PERCENT=80.0`)

### `migration_check`
- поднимает `postgres` в CI
- прогоняет SQL-миграции через `cmd/migrate`:
  - `up`
  - `down -steps 1`
  - повторный `up`
- запускает PostgreSQL integration tests для `IdempotencyRepository`
- сохраняет артефакт `migration-check-logs`

### `build`
- сборка `bin/order-service` с metadata (`version/commit/date`)

### `premerge_stand` (PR + push в main/master/dev)
- выбирает профиль (`dev` / `release`) по `github.base_ref`
- поднимает стенд `docker compose` (`oms`, `zookeeper`, `kafka`, `prometheus`)
- валидирует `Prometheus` rule-файл (`promtool check rules`)
- проверяет `/healthz` + gRPC доступность (`grpcurl` probe)
- выполняет lifecycle smoke (`scripts/saga_load.sh`)
- поддерживает два режима rollout через env:
  - `STAND_SHADOW_MODE=true`
  - `STAND_ENFORCE_STRICT=false|true`
- в `shadow`-режиме риск-гейты (`outbox delivery`, `load`, `observability`) не блокируют merge, но явно отражаются в summary
- в `strict`-режиме риск-гейты блокируют merge
- при падении блокирует merge

### `docker` (push only)
- сборка Docker image

### `security`
- gosec scan + SARIF upload

### `summary`
- сводка статусов и итоговый verdict pipeline

---

## Профили pre-merge стенда

`dev` профиль:
- `ITERATIONS=40`
- `CANCEL_RATE=20`
- `MAX_ERROR_RATE=0.015`
- `MAX_P95_MS=700`
- `MAX_AVG_MS=450`
- `TIMEOUT=8s`
- `TOTAL=250`
- `CONCURRENCY=25`
- `CONNECTIONS=12`

`release` профиль (`main` / `master`):
- `ITERATIONS=120`
- `CANCEL_RATE=25`
- `MAX_ERROR_RATE=0.010`
- `MAX_P95_MS=950`
- `MAX_AVG_MS=650`
- `TIMEOUT=8s`
- `TOTAL=400`
- `CONCURRENCY=40`
- `CONNECTIONS=20`

---

## Merge Gate Policy

Рекомендуемые required checks в Branch Protection для `dev` и `main`:
- `Lint & Format`
- `Tests`
- `Migration Check`
- `Build`
- `Pre-Merge Stand`

Без required checks merge-gate не считается жёстким.

---

## Локальный прогон перед PR

```bash
# Полный локальный CI-эквивалент ключевых gate'ов
make ci-local

# Локальный стенд (включая Prometheus)
docker compose up -d --build oms zookeeper kafka prometheus
make wait-health

# Smoke
ITERATIONS=40 CANCEL_RATE=20 ./scripts/saga_load.sh

# Outbox delivery gate
LOG_FILE=/tmp/outbox-delivery-gate.log ./scripts/ci/outbox_delivery_gate.sh

# Load gate (dev профиль)
MODE=create-pay-cancel CANCEL_RATE=20 TIMEOUT=8s MAX_ERROR_RATE=0.015 MAX_P95_MS=700 MAX_AVG_MS=450 TOTAL=250 CONCURRENCY=25 CONNECTIONS=12 OUT=/tmp/load-gate-report.json ./scripts/ci/load_gate.sh

# Observability gate
CHECK_PROMETHEUS=1 CHECK_OUTBOX_ATTEMPTS=1 METRICS_FILE=/tmp/oms-metrics.prom ./scripts/ci/observability_gate.sh

# Cleanup
docker compose down -v
```

Если нужен только тестовый gate из CI (race + coverage >= 80%), используйте:

```bash
make ci-test-gate
```

---

## Отладка падения `premerge_stand`

1. Проверить артефакт `premerge-stand-artifacts` в GitHub Actions.
2. Локально воспроизвести шаги из секции выше.
3. Проверить доступность `/healthz`, `/livez`, `/readyz`, `/metrics`, наличие бизнес-метрик `oms_*`, статус scrape в Prometheus (`up{job="oms"} == 1`) и соблюдение latency/error порогов.
4. Если `observability_gate` флапает сразу после smoke, увеличить `METRICS_SETTLE_TIMEOUT` (по умолчанию 20с) для ожидания асинхронных saga-обновлений метрик.

---

## Nightly reliability workflow

Workflow: `.github/workflows/nightly-stand.yml` (фаза N1: только manual run)

Что делает:
- поднимает stand в `postgres`-режиме;
- выполняет smoke + `outbox delivery gate`;
- запускает долгий load gate (`DURATION=12m`);
- выполняет chaos checks:
  - кратковременный outage Kafka + проверка восстановления доставки;
  - кратковременный outage Postgres + проверка восстановления readiness/gRPC;
- публикует артефакты nightly-прогона;
- строит trend report (`last N`) по метрикам load gate из предыдущих CI артефактов.
- переход на cron schedule делается после 2 успешных manual прогонов подряд.

---

## Дополнительные ресурсы

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Prometheus Query API](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [Gosec](https://github.com/securego/gosec)
