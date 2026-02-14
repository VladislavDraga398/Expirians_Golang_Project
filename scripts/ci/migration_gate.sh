#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Required tool not found: $1"
    exit 1
  }
}

need docker
need go

POSTGRES_USER=${POSTGRES_USER:-oms}
POSTGRES_DB=${POSTGRES_DB:-oms}
POSTGRES_PORT=${POSTGRES_PORT:-55432}
OMS_POSTGRES_DSN=${OMS_POSTGRES_DSN:-postgres://oms:oms@localhost:${POSTGRES_PORT}/oms?sslmode=disable}
POSTGRES_LOG_FILE=${POSTGRES_LOG_FILE:-postgres-migration-logs.txt}

cleanup() {
  docker compose logs postgres --tail=200 > "$POSTGRES_LOG_FILE" 2>/dev/null || true
  docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "Starting postgres for migration gate"
POSTGRES_PORT="$POSTGRES_PORT" docker compose up -d postgres

echo "Waiting for postgres readiness"
for i in $(seq 1 120); do
  if docker compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
    echo "Postgres is ready"
    break
  fi
  sleep 1
done

if ! docker compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
  echo "Postgres did not become ready in time"
  exit 1
fi

echo "Apply all up migrations"
OMS_POSTGRES_DSN="$OMS_POSTGRES_DSN" go run ./cmd/migrate -direction up

echo "Rollback last migration"
OMS_POSTGRES_DSN="$OMS_POSTGRES_DSN" go run ./cmd/migrate -direction down -steps 1

echo "Re-apply migrations after rollback"
OMS_POSTGRES_DSN="$OMS_POSTGRES_DSN" go run ./cmd/migrate -direction up

echo "Migration status"
status_output="$(OMS_POSTGRES_DSN="$OMS_POSTGRES_DSN" go run ./cmd/migrate -direction status)"
echo "$status_output"

if ! echo "$status_output" | grep -Eq 'applied=[1-9][0-9]*'; then
  echo "Unexpected migration status after re-apply: $status_output"
  exit 1
fi

echo "Run PostgreSQL repository integration tests"
if ! OMS_POSTGRES_TEST_DSN="$OMS_POSTGRES_DSN" go test ./internal/storage/postgres -run '^TestIdempotencyRepository_Postgres' -count=1 -v; then
  echo "PostgreSQL integration tests failed on first attempt, retrying once after readiness re-check"
  for i in $(seq 1 30); do
    if docker compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  OMS_POSTGRES_TEST_DSN="$OMS_POSTGRES_DSN" go test ./internal/storage/postgres -run '^TestIdempotencyRepository_Postgres' -count=1 -v
fi

echo "Migration gate passed"
