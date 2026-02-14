#!/usr/bin/env bash
set -euo pipefail

# Config
PROTO="proto/oms/v1/order_service.proto"
IMPORT_PATHS=(-import-path . -import-path proto)
ADDR="localhost:50051"
CURRENCY="USD"
RUN_ID="$(date +%s%N)-$RANDOM"

# Helpers
need() { command -v "$1" >/dev/null 2>&1 || { echo "Error: '$1' not found in PATH"; exit 1; }; }

extract_order_id() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.order.id' | sed 's/null//g'
  else
    sed -n 's/.*"id"\s*:\s*"\([^"]\+\)".*/\1/p' | head -n1
  fi
}

extract_order_status() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.order.status'
  else
    sed -n 's/.*"status"\s*:\s*"\([^"]\+\)".*/\1/p' | head -n1
  fi
}

wait_for_status() {
  local order_id="$1"
  local expected="$2"
  local attempts="${3:-30}"
  local sleep_seconds="${4:-0.2}"
  local current=""

  for _ in $(seq 1 "$attempts"); do
    local resp
    resp=$(grpcurl -plaintext \
      "${IMPORT_PATHS[@]}" \
      -proto "$PROTO" \
      -d '{"order_id":"'"$order_id"'"}' \
      "$ADDR" oms.v1.OrderService/GetOrder)
    current=$(echo "$resp" | extract_order_status)
    if [[ "$current" == "$expected" ]]; then
      echo "$resp"
      return 0
    fi
    sleep "$sleep_seconds"
  done

  echo "Timed out waiting for status $expected (current: ${current:-unknown})" >&2
  return 1
}

banner() { echo; echo "==== $* ===="; }

need grpcurl

banner "CreateOrder"
CREATE_RESP=$(grpcurl -plaintext \
  "${IMPORT_PATHS[@]}" \
  -H "idempotency-key: demo-success-create-${RUN_ID}" \
  -proto "$PROTO" \
  -d '{
        "customer_id":"cust-success",
        "currency":"'$CURRENCY'",
        "items":[{"sku":"sku-success","qty":1,"price":{"currency":"'$CURRENCY'","amount_minor":1000}}]
      }' \
  "$ADDR" oms.v1.OrderService/CreateOrder)

echo "$CREATE_RESP" | sed 's/.*/  &/'
ORDER_ID=$(echo "$CREATE_RESP" | extract_order_id)
if [[ -z "${ORDER_ID}" ]]; then
  echo "Failed to extract order id" >&2
  exit 1
fi

echo "OrderID: $ORDER_ID"

banner "PayOrder (triggers saga)"
grpcurl -plaintext \
  "${IMPORT_PATHS[@]}" \
  -H "idempotency-key: demo-success-pay-${ORDER_ID}-${RUN_ID}" \
  -proto "$PROTO" \
  -d '{"order_id":"'$ORDER_ID'"}' \
  "$ADDR" oms.v1.OrderService/PayOrder | sed 's/.*/  &/'

banner "GetOrder (should be CONFIRMED)"
wait_for_status "$ORDER_ID" "ORDER_STATUS_CONFIRMED" | sed 's/.*/  &/'

echo
echo "Success scenario complete. Order should be CONFIRMED."
echo "Check metrics: oms_saga_completed_total should be > 0"
echo "Tip: open Grafana http://localhost:3000 (admin/admin) → OMS → 'OMS Saga Overview'"
