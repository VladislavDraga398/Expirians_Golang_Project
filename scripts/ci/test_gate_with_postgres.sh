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

POSTGRES_USER="${POSTGRES_USER:-oms}"
POSTGRES_DB="${POSTGRES_DB:-oms}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
OMS_POSTGRES_DSN="${OMS_POSTGRES_DSN:-postgres://oms:oms@localhost:${POSTGRES_PORT}/oms?sslmode=disable}"

cleanup() {
  if [[ "${CI_KEEP_POSTGRES:-0}" == "1" ]]; then
    return
  fi
  docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "Starting postgres for test gate"
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

OMS_POSTGRES_TEST_DSN="$OMS_POSTGRES_DSN" OMS_POSTGRES_DSN="$OMS_POSTGRES_DSN" ./scripts/ci/test_gate.sh
