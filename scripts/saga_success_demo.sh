#!/usr/bin/env bash
set -euo pipefail

# Config
PROTO="proto/oms/v1/order_service.proto"
ADDR="localhost:50051"
CURRENCY="USD"

# Helpers
need() { command -v "$1" >/dev/null 2>&1 || { echo "Error: '$1' not found in PATH"; exit 1; }; }

extract_order_id() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.order.id' | sed 's/null//g'
  else
    sed -n 's/.*"id"\s*:\s*"\([^"]\+\)".*/\1/p' | head -n1
  fi
}

banner() { echo; echo "==== $* ===="; }

need grpcurl

banner "CreateOrder"
CREATE_RESP=$(grpcurl -plaintext \
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
  -proto "$PROTO" \
  -d '{"order_id":"'$ORDER_ID'"}' \
  "$ADDR" oms.v1.OrderService/PayOrder | sed 's/.*/  &/'

banner "GetOrder (should be CONFIRMED)"
grpcurl -plaintext \
  -proto "$PROTO" \
  -d '{"order_id":"'$ORDER_ID'"}' \
  "$ADDR" oms.v1.OrderService/GetOrder | sed 's/.*/  &/'

echo
echo "Success scenario complete. Order should be CONFIRMED."
echo "Check metrics: oms_saga_completed_total should be > 0"
echo "Tip: open Grafana http://localhost:3000 (admin/admin) → OMS → 'OMS Saga Overview'"
