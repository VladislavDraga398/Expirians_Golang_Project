#!/usr/bin/env bash
set -euo pipefail

# Config
PROTO="proto/oms/v1/order_service.proto"
ADDR="localhost:50051"
CURRENCY="USD"
REASON="Customer changed mind"

# Helpers
need() { command -v "$1" >/dev/null 2>&1 || { echo "Error: '$1' not found in PATH"; exit 1; }; }

extract_order_id() {
  # Prefer jq if available
  if command -v jq >/dev/null 2>&1; then
    jq -r '.order.id' | sed 's/null//g'
  else
    # Fallback: naive grep/sed extraction
    # expects JSON like: {"order":{"id":"..." ...}}
    sed -n 's/.*"id"\s*:\s*"\([^"]\+\)".*/\1/p' | head -n1
  fi
}

banner() { echo; echo "==== $* ===="; }

need grpcurl

banner "CreateOrder"
CREATE_RESP=$(grpcurl -plaintext \
  -proto "$PROTO" \
  -d '{
        "customer_id":"cust-1",
        "currency":"'$CURRENCY'",
        "items":[{"sku":"sku-1","qty":1,"price":{"currency":"'$CURRENCY'","amount_minor":1000}}]
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
  -proto "$PROTO" \
  -d '{"order_id":"'$ORDER_ID'"}' \
  "$ADDR" oms.v1.OrderService/PayOrder | sed 's/.*/  &/'

banner "GetOrder"
GET_RESP=$(grpcurl -plaintext \
  -proto "$PROTO" \
  -d '{"order_id":"'$ORDER_ID'"}' \
  "$ADDR" oms.v1.OrderService/GetOrder)

echo "$GET_RESP" | sed 's/.*/  &/'

banner "CancelOrder (compensation)"
if command -v jq >/dev/null 2>&1; then
  JSON_PAYLOAD=$(jq -n --arg id "$ORDER_ID" --arg reason "$REASON" '{order_id:$id, reason:$reason}')
else
  JSON_PAYLOAD=$(printf '{"order_id":"%s","reason":"%s"}' "$ORDER_ID" "$REASON")
fi
grpcurl -plaintext \
  -proto "$PROTO" \
  -d "$JSON_PAYLOAD" \
  "$ADDR" oms.v1.OrderService/CancelOrder | sed 's/.*/  &/'

banner "GetOrder after cancel"
grpcurl -plaintext \
  -proto "$PROTO" \
  -d '{"order_id":"'$ORDER_ID'"}' \
  "$ADDR" oms.v1.OrderService/GetOrder | sed 's/.*/  &/'

echo
echo "Tip: open Grafana http://localhost:3000 (admin/admin) → OMS → 'OMS Saga Overview'"
echo "      and Prometheus http://localhost:9091 to see metrics update."
