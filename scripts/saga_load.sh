#!/usr/bin/env bash
set -euo pipefail

# Config
PROTO="proto/oms/v1/order_service.proto"
ADDR="localhost:50051"
CURRENCY="USD"
ITERATIONS=${ITERATIONS:-30}
CANCEL_RATE=${CANCEL_RATE:-0}   # 0..100 (процент отмен после оплаты)

need() { command -v "$1" >/dev/null 2>&1 || { echo "Error: '$1' not found in PATH"; exit 1; }; }
extract_order_id() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.order.id' | sed 's/null//g'
  else
    sed -n 's/.*"id"\s*:\s*"\([^"]\+\)".*/\1/p' | head -n1
  fi
}

need grpcurl

printf "Running saga load: iterations=%d, cancel_rate=%d%%\n" "$ITERATIONS" "$CANCEL_RATE"

for i in $(seq 1 "$ITERATIONS"); do
  # Create
  CREATE_RESP=$(grpcurl -plaintext \
    -proto "$PROTO" \
    -d '{
          "customer_id":"load-'"$i"'",
          "currency":"'$CURRENCY'",
          "items":[{"sku":"sku-1","qty":1,"price":{"currency":"'$CURRENCY'","amount_minor":1000}}]
        }' \
    "$ADDR" oms.v1.OrderService/CreateOrder)
  ORDER_ID=$(echo "$CREATE_RESP" | extract_order_id)
  if [[ -z "${ORDER_ID}" ]]; then
    echo "[#$i] failed to get order id" >&2
    continue
  fi

  # Pay (triggers saga)
  grpcurl -plaintext \
    -proto "$PROTO" \
    -d '{"order_id":"'$ORDER_ID'"}' \
    "$ADDR" oms.v1.OrderService/PayOrder >/dev/null

  # Optional Cancel based on rate
  if [[ "$CANCEL_RATE" -gt 0 ]]; then
    r=$((RANDOM % 100))
    if [[ "$r" -lt "$CANCEL_RATE" ]]; then
      grpcurl -plaintext \
        -proto "$PROTO" \
        -d '{"order_id":"'$ORDER_ID'", "reason":"load-cancel"}' \
        "$ADDR" oms.v1.OrderService/CancelOrder >/dev/null || true
    fi
  fi

done

echo "Done. Check Grafana (OMS Saga Overview)."
