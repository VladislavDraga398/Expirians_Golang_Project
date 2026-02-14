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

float_lt() {
  local left="$1"
  local right="$2"
  awk -v l="$left" -v r="$right" 'BEGIN { exit !(l < r) }'
}

metric_sum() {
  local metric="$1"
  local file="$2"
  awk -v m="$metric" '
    $1 ~ ("^" m "(\\{|$)") { sum += $2; found = 1 }
    END {
      if (!found) {
        print "0"
      } else {
        printf "%.6f", sum
      }
    }
  ' "$file"
}

require_metric() {
  local metric="$1"
  local file="$2"
  grep -Eq "^${metric}(\\{| )" "$file" || {
    echo "Missing metric: $metric"
    return 1
  }
}

check_http_200() {
  local url="$1"
  local name="$2"
  local code

  if ! code=$(curl -sS -o /dev/null -w "%{http_code}" "$url"); then
    echo "${name} check failed: cannot reach $url"
    return 1
  fi
  if [[ "$code" != "200" ]]; then
    echo "${name} check failed: HTTP $code ($url)"
    return 1
  fi

  echo "${name}: HTTP 200"
}

need curl
need awk
need grep
need jq

HEALTH_URL=${HEALTH_URL:-http://localhost:9090/healthz}
LIVE_URL=${LIVE_URL:-http://localhost:9090/livez}
READY_URL=${READY_URL:-http://localhost:9090/readyz}
METRICS_URL=${METRICS_URL:-http://localhost:9090/metrics}
PROM_URL=${PROM_URL:-http://localhost:9091}

CHECK_PROMETHEUS=${CHECK_PROMETHEUS:-0}
PROM_READY_TIMEOUT=${PROM_READY_TIMEOUT:-60}
PROM_SCRAPE_TIMEOUT=${PROM_SCRAPE_TIMEOUT:-45}

METRICS_FILE=${METRICS_FILE:-/tmp/oms-metrics.prom}

echo "Running observability gate"
echo "health=$HEALTH_URL live=$LIVE_URL ready=$READY_URL metrics=$METRICS_URL"

fail=0

check_http_200 "$HEALTH_URL" "healthz" || fail=1
check_http_200 "$LIVE_URL" "livez" || fail=1
check_http_200 "$READY_URL" "readyz" || fail=1

if ! curl -fsS "$METRICS_URL" -o "$METRICS_FILE"; then
  echo "Failed to fetch metrics from $METRICS_URL"
  fail=1
else
  echo "metrics snapshot saved to $METRICS_FILE"
fi

required_metrics=(
  "go_goroutines"
  "process_start_time_seconds"
  "oms_saga_started_total"
  "oms_saga_completed_total"
  "oms_saga_canceled_total"
  "oms_saga_failed_total"
  "oms_active_sagas"
  "oms_saga_duration_seconds_count"
  "oms_timeline_events_total"
  "oms_outbox_events_total"
)

if [[ "$fail" -eq 0 ]]; then
  for metric in "${required_metrics[@]}"; do
    require_metric "$metric" "$METRICS_FILE" || fail=1
  done
fi

if [[ "$fail" -eq 0 ]]; then
  saga_started=$(metric_sum "oms_saga_started_total" "$METRICS_FILE")
  saga_completed=$(metric_sum "oms_saga_completed_total" "$METRICS_FILE")
  saga_canceled=$(metric_sum "oms_saga_canceled_total" "$METRICS_FILE")
  saga_failed=$(metric_sum "oms_saga_failed_total" "$METRICS_FILE")
  active_sagas=$(metric_sum "oms_active_sagas" "$METRICS_FILE")
  saga_duration_count=$(metric_sum "oms_saga_duration_seconds_count" "$METRICS_FILE")
  timeline_events=$(metric_sum "oms_timeline_events_total" "$METRICS_FILE")

  terminal_sagas=$(awk -v c="$saga_completed" -v k="$saga_canceled" -v f="$saga_failed" 'BEGIN { printf "%.6f", c + k + f }')

  echo "Observability counters"
  echo "saga_started:   $saga_started"
  echo "saga_terminal:  $terminal_sagas"
  echo "saga_duration:  $saga_duration_count"
  echo "timeline_events:$timeline_events"
  echo "active_sagas:   $active_sagas"

  if ! float_gt "$saga_started" "0"; then
    echo "oms_saga_started_total must be > 0"
    fail=1
  fi
  if ! float_gt "$terminal_sagas" "0"; then
    echo "Terminal saga counters must be > 0"
    fail=1
  fi
  if ! float_gt "$saga_duration_count" "0"; then
    echo "oms_saga_duration_seconds_count must be > 0"
    fail=1
  fi
  if ! float_gt "$timeline_events" "0"; then
    echo "oms_timeline_events_total must be > 0"
    fail=1
  fi
  if float_lt "$active_sagas" "0"; then
    echo "oms_active_sagas is negative ($active_sagas), investigate saga gauge consistency"
  fi
fi

if [[ "$CHECK_PROMETHEUS" = "1" ]]; then
  echo "Verifying Prometheus scrape path via $PROM_URL"

  ready=0
  for i in $(seq 1 "$PROM_READY_TIMEOUT"); do
    if curl -fsS "$PROM_URL/-/ready" >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done

  if [[ "$ready" -ne 1 ]]; then
    echo "Prometheus is not ready in ${PROM_READY_TIMEOUT}s"
    fail=1
  else
    echo "Prometheus is ready"
  fi

  if [[ "$ready" -eq 1 ]]; then
    up_ok=0
    for i in $(seq 1 "$PROM_SCRAPE_TIMEOUT"); do
      up_value=$(curl -fsS --get \
        --data-urlencode 'query=up{job="oms"}' \
        "$PROM_URL/api/v1/query" | jq -r '.data.result[0].value[1] // "0"')
      if [[ "$up_value" = "1" || "$up_value" = "1.0" ]]; then
        up_ok=1
        break
      fi
      sleep 1
    done

    if [[ "$up_ok" -ne 1 ]]; then
      echo "Prometheus scrape for job=oms is not healthy (up != 1)"
      fail=1
    else
      echo "Prometheus scrape is healthy (up=1)"
    fi
  fi
fi

if [[ "$fail" -ne 0 ]]; then
  echo "Observability gate failed"
  exit 1
fi

echo "Observability gate passed"
