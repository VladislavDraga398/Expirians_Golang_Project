# CI/CD Pipeline

Автоматизированный pipeline для проверки, тестирования и merge-gate проекта OMS.

**Версия:** v2.2 | **Обновлено:** 2026-02-12

---

## TL;DR
- Тестовый стенд не хранится в отдельной ветке: он поднимается как временное окружение в GitHub Actions.
- `premerge_stand` запускается на каждом PR, но с разными профилями по целевой ветке.
- Merge разрешён только после зелёных `lint`, `test`, `build`, `premerge_stand`.
- SQL-модель дополнительно защищена обязательным `migration_check`.

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
- coverage report

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

### `premerge_stand` (PR only)
- выбирает профиль (`dev` / `release`) по `github.base_ref`
- поднимает стенд `docker compose` (`oms`, `zookeeper`, `kafka`, `prometheus`)
- проверяет `/healthz`
- выполняет lifecycle smoke (`scripts/saga_load.sh`)
- выполняет observability gate (`scripts/ci/observability_gate.sh`)
- выполняет load gate (`scripts/ci/load_gate.sh`, внутри запускается `go run ./cmd/loadtest`)
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
- `MAX_ERROR_RATE=0.020`
- `MAX_P95_MS=500`
- `MAX_AVG_MS=180`
- `TOTAL=200`
- `CONCURRENCY=20`
- `CONNECTIONS=10`

`release` профиль (`main` / `master`):
- `ITERATIONS=120`
- `CANCEL_RATE=25`
- `MAX_ERROR_RATE=0.010`
- `MAX_P95_MS=350`
- `MAX_AVG_MS=120`
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
# Базовые проверки
make fmt
make lint
make test-race
OMS_POSTGRES_DSN='postgres://oms:oms@localhost:55432/oms?sslmode=disable' ./scripts/ci/migration_gate.sh
make build

# Локальный стенд (включая Prometheus)
docker compose up -d --build oms zookeeper kafka prometheus
make wait-health

# Smoke
ITERATIONS=40 CANCEL_RATE=20 ./scripts/saga_load.sh

# Observability gate
CHECK_PROMETHEUS=1 ./scripts/ci/observability_gate.sh

# Load gate (dev профиль)
MAX_ERROR_RATE=0.020 MAX_P95_MS=500 MAX_AVG_MS=180 TOTAL=200 CONCURRENCY=20 CONNECTIONS=10 ./scripts/ci/load_gate.sh

# Cleanup
docker compose down -v
```

---

## Отладка падения `premerge_stand`

1. Проверить артефакт `premerge-stand-logs` в GitHub Actions.
2. Локально воспроизвести шаги из секции выше.
3. Проверить доступность `/healthz`, `/livez`, `/readyz`, `/metrics`, наличие бизнес-метрик `oms_*`, статус scrape в Prometheus (`up{job="oms"} == 1`) и соблюдение latency/error порогов.

---

## Дополнительные ресурсы

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Prometheus Query API](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [Gosec](https://github.com/securego/gosec)
