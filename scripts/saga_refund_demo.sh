#!/usr/bin/env bash
set -euo pipefail

# Config
PROTO="proto/oms/v1/order_service.proto"
IMPORT_PATHS=(-import-path . -import-path proto)
ADDR="localhost:50051"
CURRENCY="USD"
REFUND_MINOR=500
REASON="Partial refund"
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
  -H "idempotency-key: demo-refund-create-${RUN_ID}" \
  -proto "$PROTO" \
  -d '{
        "customer_id":"cust-2",
        "currency":"'$CURRENCY'",
        "items":[{"sku":"sku-2","qty":1,"price":{"currency":"'$CURRENCY'","amount_minor":1000}}]
      }' \
  "$ADDR" oms.v1.OrderService/CreateOrder)

echo "$CREATE_RESP" | sed 's/.*/  &/'
ORDER_ID=$(echo "$CREATE_RESP" | extract_order_id)
if [[ -z "${ORDER_ID}" ]]; then
  echo "Failed to extract order id" >&2
  exit 1
fi

echo "OrderID: $ORDER_ID"

banner "PayOrder"
grpcurl -plaintext \
  "${IMPORT_PATHS[@]}" \
  -H "idempotency-key: demo-refund-pay-${ORDER_ID}-${RUN_ID}" \
  -proto "$PROTO" \
  -d '{"order_id":"'$ORDER_ID'"}' \
  "$ADDR" oms.v1.OrderService/PayOrder | sed 's/.*/  &/'

wait_for_status "$ORDER_ID" "ORDER_STATUS_CONFIRMED" >/dev/null

banner "RefundOrder (partial)"
if command -v jq >/dev/null 2>&1; then
  JSON_PAYLOAD=$(jq -n --arg id "$ORDER_ID" --arg cur "$CURRENCY" --argjson amt $REFUND_MINOR '{order_id:$id, amount:{currency:$cur, amount_minor:$amt}, reason:"Partial refund"}')
else
  JSON_PAYLOAD=$(printf '{"order_id":"%s","amount":{"currency":"%s","amount_minor":%d},"reason":"%s"}' "$ORDER_ID" "$CURRENCY" "$REFUND_MINOR" "$REASON")
fi

grpcurl -plaintext \
  "${IMPORT_PATHS[@]}" \
  -H "idempotency-key: demo-refund-${ORDER_ID}-${RUN_ID}" \
  -proto "$PROTO" \
  -d "$JSON_PAYLOAD" \
  "$ADDR" oms.v1.OrderService/RefundOrder | sed 's/.*/  &/'

banner "GetOrder after refund"
wait_for_status "$ORDER_ID" "ORDER_STATUS_REFUNDED" | sed 's/.*/  &/'

echo
echo "Tip: open Grafana http://localhost:3000 (admin/admin) → OMS → 'OMS Saga Overview'"
echo "      and Prometheus http://localhost:9091 to see metrics update."
