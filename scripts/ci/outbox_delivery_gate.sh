#!/usr/bin/env bash
set -euo pipefail

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Required tool not found: $1"
    exit 1
  }
}

need grpcurl
need jq
need docker

PROTO=${PROTO:-proto/oms/v1/order_service.proto}
ADDR=${ADDR:-localhost:50051}
TOPIC=${TOPIC:-oms.order.events}
KAFKA_SERVICE=${KAFKA_SERVICE:-kafka}
KAFKA_BROKER=${KAFKA_BROKER:-kafka:9092}
CONSUMER_TIMEOUT_MS=${CONSUMER_TIMEOUT_MS:-45000}
MAX_MESSAGES=${MAX_MESSAGES:-200000}
FALLBACK_TIMEOUT_MS=${FALLBACK_TIMEOUT_MS:-180000}
FALLBACK_MAX_MESSAGES=${FALLBACK_MAX_MESSAGES:-$MAX_MESSAGES}
CONSUMER_WARMUP_SEC=${CONSUMER_WARMUP_SEC:-5}
OUTPUT_PREVIEW_LINES=${OUTPUT_PREVIEW_LINES:-200}
LOG_FILE=${LOG_FILE:-/tmp/outbox-delivery-gate.log}

RUN_ID=${RUN_ID:-"stand-$(date +%s%N)-$RANDOM"}
CUSTOMER_ID=${CUSTOMER_ID:-"outbox-gate-${RUN_ID}"}

consumer_stdout="$(mktemp)"
consumer_stderr="$(mktemp)"
fallback_stdout="$(mktemp)"
fallback_stderr="$(mktemp)"
create_resp_file="$(mktemp)"
pay_resp_file="$(mktemp)"

cleanup() {
  rm -f "$consumer_stdout" "$consumer_stderr" "$fallback_stdout" "$fallback_stderr" "$create_resp_file" "$pay_resp_file"
}
trap cleanup EXIT

append_preview() {
  local title="$1"
  local path="$2"
  local line_count
  line_count=$(wc -l <"$path" | awk '{print $1}')
  echo "=== ${title} (lines=${line_count}) ==="
  if [[ "$line_count" -eq 0 ]]; then
    echo "<empty>"
    return
  fi
  sed -n "1,${OUTPUT_PREVIEW_LINES}p" "$path"
  if [[ "$line_count" -gt "$OUTPUT_PREVIEW_LINES" ]]; then
    echo "... truncated after ${OUTPUT_PREVIEW_LINES} lines ..."
  fi
}

echo "Running outbox delivery gate"
echo "grpc_addr=$ADDR topic=$TOPIC kafka_service=$KAFKA_SERVICE"

grpcurl -plaintext \
  -import-path . \
  -import-path proto \
  -H "idempotency-key: outbox-gate-create-${RUN_ID}" \
  -proto "$PROTO" \
  -d '{
    "customer_id":"'"$CUSTOMER_ID"'",
    "currency":"USD",
    "items":[{"sku":"sku-outbox-gate","qty":1,"price":{"currency":"USD","amount_minor":1000}}]
  }' \
  "$ADDR" oms.v1.OrderService/CreateOrder >"$create_resp_file"

order_id=$(jq -r '.order.id // empty' "$create_resp_file")
if [[ -z "$order_id" ]]; then
  echo "Failed to parse order ID from CreateOrder response"
  cat "$create_resp_file"
  exit 1
fi

echo "Created order_id=$order_id"

# Start consumer before triggering PayOrder to avoid missing the publish window.
docker compose exec -T "$KAFKA_SERVICE" \
  kafka-console-consumer \
  --bootstrap-server "$KAFKA_BROKER" \
  --topic "$TOPIC" \
  --timeout-ms "$CONSUMER_TIMEOUT_MS" \
  --max-messages "$MAX_MESSAGES" \
  --property print.key=true \
  --property key.separator="|" >"$consumer_stdout" 2>"$consumer_stderr" &
consumer_pid=$!

sleep "$CONSUMER_WARMUP_SEC"

grpcurl -plaintext \
  -import-path . \
  -import-path proto \
  -H "idempotency-key: outbox-gate-pay-${RUN_ID}" \
  -proto "$PROTO" \
  -d '{"order_id":"'"$order_id"'"}' \
  "$ADDR" oms.v1.OrderService/PayOrder >"$pay_resp_file"

if ! wait "$consumer_pid"; then
  echo "Kafka consumer exited with non-zero status; continuing with payload verification"
fi

found=0
if grep -Fq "${order_id}|" "$consumer_stdout"; then
  found=1
elif grep -Fq "\"aggregate_id\":\"${order_id}\"" "$consumer_stdout"; then
  found=1
elif grep -Fq "\"order_id\":\"${order_id}\"" "$consumer_stdout"; then
  found=1
fi

if [[ "$found" -ne 1 ]]; then
  echo "Primary consume window did not include order_id=$order_id, running fallback scan from beginning"
  docker compose exec -T "$KAFKA_SERVICE" \
    kafka-console-consumer \
    --bootstrap-server "$KAFKA_BROKER" \
    --topic "$TOPIC" \
    --from-beginning \
    --timeout-ms "$FALLBACK_TIMEOUT_MS" \
    --max-messages "$FALLBACK_MAX_MESSAGES" \
    --property print.key=true \
    --property key.separator="|" >"$fallback_stdout" 2>"$fallback_stderr" || true

  if grep -Fq "${order_id}|" "$fallback_stdout"; then
    found=1
  elif grep -Fq "\"aggregate_id\":\"${order_id}\"" "$fallback_stdout"; then
    found=1
  elif grep -Fq "\"order_id\":\"${order_id}\"" "$fallback_stdout"; then
    found=1
  fi
fi

{
  echo "run_id=$RUN_ID"
  echo "order_id=$order_id"
  echo "topic=$TOPIC"
  echo "grpc_addr=$ADDR"
  echo "consumer_timeout_ms=$CONSUMER_TIMEOUT_MS"
  echo "max_messages=$MAX_MESSAGES"
  echo "fallback_timeout_ms=$FALLBACK_TIMEOUT_MS"
  echo "fallback_max_messages=$FALLBACK_MAX_MESSAGES"
  echo "consumer_warmup_sec=$CONSUMER_WARMUP_SEC"
  echo "preview_lines=$OUTPUT_PREVIEW_LINES"
  echo
  echo "=== CreateOrder response ==="
  cat "$create_resp_file"
  echo
  echo "=== PayOrder response ==="
  cat "$pay_resp_file"
  echo
  append_preview "Consumer stdout (live)" "$consumer_stdout"
  echo
  append_preview "Consumer stderr (live)" "$consumer_stderr"
  echo
  append_preview "Consumer stdout (fallback from beginning)" "$fallback_stdout"
  echo
  append_preview "Consumer stderr (fallback from beginning)" "$fallback_stderr"
  echo
  echo "=== Matches by order_id ==="
  grep -nF "$order_id" "$consumer_stdout" || true
  grep -nF "$order_id" "$fallback_stdout" || true
} >"$LOG_FILE"

if [[ "$found" -ne 1 ]]; then
  echo "Outbox delivery gate failed: no Kafka message found for order_id=$order_id"
  echo "See log: $LOG_FILE"
  exit 1
fi

echo "Outbox delivery gate passed (order_id=$order_id)"
echo "Log saved to $LOG_FILE"
