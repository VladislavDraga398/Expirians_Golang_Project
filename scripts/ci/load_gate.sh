#!/usr/bin/env bash
set -euo pipefail

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Required tool not found: $1"
    exit 1
  }
}

float_gt() {
  local left="$1"
  local right="$2"
  awk -v l="$left" -v r="$right" 'BEGIN { exit !(l > r) }'
}

GO=${GO:-go}
RUNNER=${RUNNER:-./cmd/loadtest}

need "$GO"
need jq

ADDR=${ADDR:-localhost:50051}
MODE=${MODE:-create}
TOTAL=${TOTAL:-400}
CONCURRENCY=${CONCURRENCY:-40}
CONNECTIONS=${CONNECTIONS:-20}
TIMEOUT=${TIMEOUT:-5s}
CANCEL_RATE=${CANCEL_RATE:-0}
OUT=${OUT:-/tmp/load-gate-report.json}

MAX_ERROR_RATE=${MAX_ERROR_RATE:-0.010}
MAX_P95_MS=${MAX_P95_MS:-350}
MAX_AVG_MS=${MAX_AVG_MS:-120}

echo "Running load gate against $ADDR"
echo "mode=$MODE n=$TOTAL c=$CONCURRENCY connections=$CONNECTIONS timeout=$TIMEOUT"

set +e
"$GO" run "$RUNNER" \
  -addr "$ADDR" \
  -mode "$MODE" \
  -total "$TOTAL" \
  -concurrency "$CONCURRENCY" \
  -connections "$CONNECTIONS" \
  -timeout "$TIMEOUT" \
  -cancel-rate "$CANCEL_RATE" \
  -output "$OUT"
load_exit=$?
set -e

if [[ ! -f "$OUT" ]]; then
  echo "load runner did not produce report: $OUT"
  exit 1
fi

count=$(jq -r '.total_scenarios // 0' "$OUT")
if [[ "$count" -le 0 ]]; then
  echo "load runner returned empty result (total_scenarios=0)"
  cat "$OUT"
  exit 1
fi

errors=$(jq -r '.failed_scenarios // 0' "$OUT")
error_rate=$(jq -r '.error_rate // 1' "$OUT")
p95_ms=$(jq -r '.scenario_latency_ms.p95 // 0' "$OUT")
avg_ms=$(jq -r '.scenario_latency_ms.avg // 0' "$OUT")
rps=$(jq -r '.rps // 0' "$OUT")

printf "\nLoad gate summary\n"
echo "count:      $count"
echo "errors:     $errors"
echo "error_rate: $error_rate"
echo "avg_ms:     $avg_ms"
echo "p95_ms:     $p95_ms"
echo "rps:        $rps"

gate_failed=0

if [[ "$load_exit" -ne 0 ]]; then
  echo "load runner exited with code $load_exit (there were scenario failures)"
  gate_failed=1
fi

if float_gt "$error_rate" "$MAX_ERROR_RATE"; then
  echo "error_rate $error_rate > max $MAX_ERROR_RATE"
  gate_failed=1
else
  echo "error_rate gate passed ($error_rate <= $MAX_ERROR_RATE)"
fi

if float_gt "$p95_ms" "$MAX_P95_MS"; then
  echo "p95_ms $p95_ms > max $MAX_P95_MS"
  gate_failed=1
else
  echo "p95 gate passed ($p95_ms <= $MAX_P95_MS)"
fi

if float_gt "$avg_ms" "$MAX_AVG_MS"; then
  echo "avg_ms $avg_ms > max $MAX_AVG_MS"
  gate_failed=1
else
  echo "avg gate passed ($avg_ms <= $MAX_AVG_MS)"
fi

printf "\nMethod summary\n"
jq -r '
  (.methods // {})
  | to_entries
  | map(select(.key != "scenario"))
  | .[]
  | "\(.key): calls=\(.value.calls // 0) success=\(.value.success // 0) failed=\(.value.failed // 0) error_rate=\(.value.error_rate // 0) p95_ms=\(.value.latency_ms.p95 // 0)"
' "$OUT"

if [[ "$gate_failed" -ne 0 ]]; then
  printf "\nLoad gate failed\n"
  exit 1
fi

printf "\nLoad gate passed\n"
