#!/usr/bin/env bash
#
# Wipe every payment and seed a fresh set that shows each state the system can
# reach. Everything goes through the real HTTP API, so each seeded payment has
# a genuine outbox event, inbox marker and gateway charge — which means the UI
# timeline and the Pay/Fail buttons work on all of them.
#
#   ./scripts/reset-and-seed.sh
#
set -euo pipefail

API="${API:-http://localhost:8080}"
GW="${GW:-http://localhost:8082}"
PSQL="docker compose exec -T postgres psql -U postgres -d design-payment-system"

say() { printf '\n\033[1m%s\033[0m\n' "$*"; }

create() { # $1=customer $2=amount $3=method -> prints payment id
  curl -s -X POST "$API/payments" \
    -H "Idempotency-Key: seed-$(date +%s%N)" \
    -H 'Content-Type: application/json' \
    -d "{\"customer_id\":\"$1\",\"amount\":$2,\"payment_method\":\"$3\"}" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["id"])'
}

settle() { curl -s -o /dev/null -X POST "$GW/charges/$1/settle" \
             -H 'Content-Type: application/json' -d "{\"status\":\"$2\"}"; }

mode()   { curl -s -o /dev/null -X PUT "$GW/config" \
             -H 'Content-Type: application/json' -d "{\"fail_mode\":\"$1\"}"; }

say "1/5  clearing payments, keys, outbox and inbox"
# Order does not matter under TRUNCATE ... CASCADE, and RESTART IDENTITY keeps
# nothing behind. schema_migrations is deliberately untouched.
$PSQL -q -c "TRUNCATE payments, idempotency_keys, outbox, processed_events RESTART IDENTITY CASCADE;"

say "2/5  clearing the gateway's charge store"
docker compose exec -T fake-gateway sh -c 'rm -f /data/charges.json' || true
docker compose restart fake-gateway >/dev/null
sleep 6
mode ""

say "3/5  three payments the customer completes"
for pair in "cust_sari 450000 qris" "cust_budi 1250000 va_bca" "cust_rina 89000 qris"; do
  set -- $pair
  id=$(create "$1" "$2" "$3"); sleep 3; settle "$id" SUCCEEDED
  echo "  SUCCEEDED  $1  $2  $3"
done

say "4/5  one the customer abandons, three still waiting"
id=$(create cust_agus 300000 va_bni); sleep 3; settle "$id" FAILED
echo "  FAILED     cust_agus (customer did not complete it)"
for pair in "cust_dewi 175000 va_bca" "cust_eko 2400000 qris" "cust_fitri 65000 card"; do
  set -- $pair
  create "$1" "$2" "$3" >/dev/null
  echo "  PROCESSING $1  $2  $3  (press Pay in the UI)"
done
# Let the worker charge these BEFORE the gateway's fail mode changes below,
# or they get declined instead of parking at PROCESSING.
sleep 6

say "5/5  the two failure paths"
mode decline; sleep 1
create cust_hadi 520000 card >/dev/null; sleep 4
echo "  FAILED     cust_hadi  — gateway declined it, no retry"

mode timeout; sleep 1
create cust_intan 980000 va_bca >/dev/null; sleep 12
echo "  UNKNOWN    cust_intan — gateway never answered, never retried"

mode ""
sleep 4

say "done"
$PSQL -c "SELECT status, count(*) FROM payments GROUP BY status ORDER BY status;"
echo "Open http://localhost:3000"
