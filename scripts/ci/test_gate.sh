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

need go

GO_CMD="${GO:-go}"
COVERAGE_MIN_PERCENT="${COVERAGE_MIN_PERCENT:-80.0}"
DEFAULT_DSN="postgres://oms:oms@localhost:5432/oms?sslmode=disable"

# Keep DSN aligned with CI test job to avoid hidden skips in postgres integration tests.
export OMS_POSTGRES_TEST_DSN="${OMS_POSTGRES_TEST_DSN:-${OMS_POSTGRES_DSN:-$DEFAULT_DSN}}"
export OMS_POSTGRES_DSN="${OMS_POSTGRES_DSN:-$OMS_POSTGRES_TEST_DSN}"

coverage_file="$(mktemp "${TMPDIR:-/tmp}/oms-coverage.XXXXXX.out")"
coverage_log="$(mktemp "${TMPDIR:-/tmp}/oms-coverage.XXXXXX.log")"
trap 'rm -f "$coverage_file" "$coverage_log"' EXIT

echo "🧪 Running tests with race detector..."
"$GO_CMD" test ./... -race -count=1 -v
echo "✅ Race tests passed"

echo "📊 Running tests with coverage..."
"$GO_CMD" test ./... -coverprofile="$coverage_file" -covermode=atomic | tee "$coverage_log" >/dev/null
"$GO_CMD" tool cover -func="$coverage_file"

total="$("$GO_CMD" tool cover -func="$coverage_file" | awk '/^total:/ {gsub("%","",$3); print $3}')"
if [[ -z "$total" ]]; then
  echo "❌ Failed to parse total coverage from profile"
  exit 1
fi

echo "Coverage total: ${total}%"
if ! awk -v total="$total" -v min="$COVERAGE_MIN_PERCENT" 'BEGIN { exit !((total + 0) >= (min + 0)) }'; then
  echo "❌ Coverage ${total}% is below required ${COVERAGE_MIN_PERCENT}%"
  if grep -q "postgres is not available for integration tests" "$coverage_log"; then
    echo "Hint: PostgreSQL integration tests were skipped. Check DSN and PostgreSQL readiness."
    echo "OMS_POSTGRES_TEST_DSN=$OMS_POSTGRES_TEST_DSN"
  fi
  exit 1
fi

echo "✅ Coverage gate passed (${total}% >= ${COVERAGE_MIN_PERCENT}%)"
