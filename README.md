# design-payment-system

Learning project that builds a payment system, step by step.

## Stack

- Go + Gin
- PostgreSQL (state + outbox)
- Kafka (event stream between the API and the worker)
- Docker Compose (every container)

## High-level flow

```
Client
  ↓
payment-service (API)
  ↓
Postgres + Outbox
  ↓
Kafka
  ↓
payment-worker
  ↓
fake-gateway (mock payment provider)
```

## Containers

| Service           | Port  | Role |
|-------------------|-------|------|
| `payment-service` | 8080  | HTTP API. Writes the payment row and its outbox row in one transaction. Relays the event to Kafka. |
| `payment-worker`  | 8081  | Consumes payment events, charges the gateway, persists the outcome. Port serves `/health` only. |
| `fake-gateway`    | 8082  | Mock payment provider. Simulates declines, latency, and timeouts. |
| `postgres`        | 5433  | Payment state and the outbox table. Host port 5433, container port 5432. |
| `kafka`           | 29092 | Event stream. `kafka:9092` inside the network, `localhost:29092` from the host. |
| `kafka-console`   | 8088  | Redpanda Console. Shows topics, messages, and headers. Dev-only, no auth. |

### Why Postgres is on host port 5433

Port 5432 on this machine belongs to a native PostgreSQL 18 from the EDB
installer (`/Library/PostgreSQL/18`, started by `/Library/LaunchDaemons/postgresql-18.plist`
at boot). Publishing there would clash with it, or connect you to the wrong
server.

So `DB_PORT=5433` in `.env` is the **host's** view: use `localhost:5433` from
psql, DBeaver, or a local `go run`. The containers ignore that mapping and
reach Postgres at `postgres:5432` over the Docker network, using the
`DB_HOST`/`DB_PORT` overrides in `docker-compose.yml`.

```bash
psql -h localhost -p 5433 -U postgres -d design-payment-system
```

## Status

Steps 1 and 2 are done:

- `POST /payments` creates a payment with status `PENDING`, keyed by the
  client's idempotency key.
- The same transaction queues a `payment.created` event in the `outbox`
  table, and a relay ships it to Kafka.

- `payment-worker` consumes `payment.created`, submits the charge to
  `fake-gateway`, and moves the payment to `PROCESSING` (or `FAILED` on a
  decline, `UNKNOWN` if the gateway never answered).
- `POST /webhook/payment` takes the gateway's signed callback and settles the
  payment to `SUCCEEDED` or `FAILED`.

Next: reconciliation, to sweep payments stuck too long.

## Walking a payment through, by hand

A charge is only an instruction. When the gateway accepts it, a virtual
account number or a QR code exists and **the customer has not paid yet**. The
payment parks at `PROCESSING` until they do — which may be never.

`fake-gateway` models that: it never settles on its own. You settle it.

```bash
# 1. create the payment -> PENDING, then PROCESSING a second later
ID=$(curl -s -X POST localhost:8080/payments \
  -H "Idempotency-Key: $(uuidgen)" -H 'Content-Type: application/json' \
  -d '{"customer_id":"cust_1","amount":275000,"payment_method":"va_bca"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')

# 2. it sits at PROCESSING for as long as you leave it
curl -s localhost:8082/charges/$ID          # what the gateway thinks

# 3. the customer pays -> the gateway signs a callback and delivers it
curl -s -X POST localhost:8082/charges/$ID/settle

# or the payment fails on their side
curl -s -X POST localhost:8082/charges/$ID/settle \
  -H 'Content-Type: application/json' -d '{"status":"FAILED"}'
```

The settle call is synchronous and returns what happened, including the HTTP
status `payment-service` gave the callback:

```json
{
  "payment_id": "...",
  "reference": "gw_...",
  "status": "SUCCEEDED",
  "webhook_event_id": "evt_...",
  "webhook_http_status": 200
}
```

The key in that URL is **your** payment id, the same value the worker sent as
`Idempotency-Key`. Real gateways echo the merchant's reference back for the
same reason: it is the only id the merchant can always resolve, including for
a charge whose response they never received.

## The webhook

`POST /webhook/payment`, signed with HMAC-SHA256 over `"<timestamp>.<body>"`,
the scheme Stripe uses.

| Header | |
|---|---|
| `X-Timestamp` | unix seconds, **inside** the signed material |
| `X-Signature` | hex digest |

Signing the timestamp is what stops replay: a captured request keeps a valid
signature forever, so the timestamp gives it a 5-minute expiry.

Three guards, each for a different failure:

| Guard | Stops |
|---|---|
| HMAC + `hmac.Equal` | forgery, and timing attacks on the comparison |
| timestamp tolerance | replay of a captured request |
| `processed_events` (`consumer = 'payment-webhook'`) | the same delivery applied twice |
| `WHERE status NOT IN ('SUCCEEDED','FAILED')` | a late delivery reopening a settled payment |

The response code is a message to the gateway:

| Code | Meaning |
|---|---|
| `200` | settled — stop retrying (sent only after the transaction commits) |
| `401` | bad signature or stale, with no detail (detail helps an attacker probe) |
| `400` | a status we do not model; it will never succeed |
| `500` | our fault — **please retry** |

## The worker

The order of operations is deliberate:

```
1. skip if already processed, or no longer PENDING
2. call the gateway       <- external, OUTSIDE any transaction
3. record the result      <- one transaction: inbox marker + status
```

Writing the database after the external call means a crash can only leave
"charged but not recorded", which the redelivered event repairs. The opposite
order leaves "recorded but not charged", and no retry fixes that, because the
inbox marker already says the event is done.

It also keeps a Postgres transaction from staying open across an HTTP call.

### Three guards

| Guard | Stops |
|---|---|
| `Idempotency-Key = payment_id` on the charge | the provider charging twice |
| `processed_events (consumer, event_id)` | the event being applied twice |
| `UPDATE ... WHERE id = $1 AND status = 'PENDING'` | two workers racing, or a stale event reopening a closed payment |

The third is a compare-and-set, cheaper than `SELECT ... FOR UPDATE` because
the check and the write are one statement.

### Decline is not failure

| Gateway answer | Worker does |
|---|---|
| accepted | `PENDING -> PROCESSING`, record the reference |
| declined (402) | `PENDING -> FAILED`, record the reference. **No retry** — a refusal is an answer. |
| timeout, 5xx, network | retry 3 times with backoff, then dead-letter. The payment stays `PENDING` and **no inbox marker is written**, so a replay reprocesses it. |

## Testing the failure paths

`fake-gateway` has a failure knob, drivable from the shell:

```bash
GATEWAY_FAIL_MODE=decline docker compose up -d fake-gateway   # 402 -> FAILED
GATEWAY_FAIL_MODE=error   docker compose up -d fake-gateway   # 500 -> retry, DLQ
GATEWAY_FAIL_MODE=timeout docker compose up -d fake-gateway   # hang -> client timeout
docker compose up -d fake-gateway                             # back to normal
```

To replay an event (this is how a dead-lettered payment recovers):

```sql
UPDATE outbox SET published_at = NULL WHERE aggregate_id = '<payment id>';
```

## The outbox

A database commit and a Kafka publish are two systems, and nothing makes them
atomic. Publishing directly leaves two bad outcomes: publish then crash before
commit (an event for a payment that does not exist), or commit then crash
before publish (a payment nobody hears about).

So `POST /payments` writes **three rows in one transaction**:

```
BEGIN
  INSERT payments           the payment itself
  CLAIM  idempotency_keys   if another request won the key, roll back
  INSERT outbox             the payment.created event
COMMIT
```

All three land or none do. A relay goroutine then polls `outbox` every second,
claims rows with `FOR UPDATE SKIP LOCKED`, publishes them, and stamps
`published_at`.

Delivery is **at-least-once**. A row already sent to Kafka can be sent again
after a crash, so consumers must deduplicate. Two details make that possible:

- The Kafka record **key is the payment id**, so every event for one payment
  lands on one partition and stays in order.
- The `event_id` header is the **stored outbox row id**, not a fresh UUID per
  attempt. A relay that generated a new id each retry would make every
  redelivery look like a new event, and consumer dedup would never fire.

Browse the topic at http://localhost:8088.

## Money and currency

This project handles **IDR only**, and treats it as zero-decimal.

`amount` is a `BIGINT` holding **whole rupiah**. `150000` means Rp 150.000.
There is no sen, because sen is obsolete and Indonesian gateways reject
decimals.

`BIGINT` is not optional here. `INT` stops at 2.147.483.647, which is about
Rp 2,1 miliar — a normal property or B2B transfer would overflow it.

Never use a float for money. `0.10` has no exact float representation.

## Create a payment

`Idempotency-Key` is a required header. `currency` is optional and defaults
to `IDR`.

```bash
curl -X POST localhost:8080/payments \
  -H 'Idempotency-Key: order-12345' \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"cust_jkt","amount":150000,"payment_method":"qris"}'
```

| Case | Status |
|---|---|
| New payment created | `201` |
| Same key, same body (a retry) | `200`, same payment id |
| Same key, different body | `409` |
| Missing `Idempotency-Key` header | `400` |
| `amount` not above 0, or currency not IDR | `400` |

### How idempotency works

The `idempotency_keys` table holds the key, a SHA-256 of the request fields,
and the payment id. The key is the primary key, so **Postgres decides the
winner**, not the application.

The payment row and the key row are written in one transaction. If another
request already holds the key, the transaction rolls back and the payment row
disappears with it. A test of 30 parallel requests with one key produces one
`201`, 29 `200`, one payment row, and zero orphans.

The key lives in its own table, not as a column on `payments`, because the two
have different lifetimes. A payment is kept for years. A key is only useful
for the client's retry window, so `expires_at` defaults to 24 hours. Nothing
deletes expired rows yet.

## Run

```bash
cp .env.example .env   # already generated, adjust if needed
make build             # docker compose up -d --build
make stop              # docker compose down
```

## Health checks

```bash
curl -s localhost:8080/health   # payment-service
curl -s localhost:8081/health   # payment-worker
curl -s localhost:8082/health   # fake-gateway
```

`payment-service` and `payment-worker` report each dependency:

```json
{ "status": "ok", "database": "ok", "kafka": "ok" }
```

`fake-gateway` depends on no infrastructure, so it reports `{"status":"ok"}`.
Any service returns `503` when a dependency it uses is unreachable.

Redpanda Console shows Kafka topics and messages at http://localhost:8088.

## Run a service locally (infrastructure in Docker)

`.env` targets the host by default, so a local `go run` reaches Postgres at
`localhost:5433` and Kafka at `localhost:29092`:

```bash
docker compose up -d postgres kafka
go run ./cmd/payment-service
```

## Layout

```
cmd/payment-service    API entrypoint
cmd/payment-worker     Worker entrypoint
cmd/fake-gateway       Mock gateway entrypoint
internal/config        Env-driven config, resolved once at startup
internal/server        Gin engine + lifecycle
internal/health        Health feature (handler -> service -> repository)
internal/payment       Payment feature (handler -> service -> repository)
internal/idempotency   Idempotency keys (the HTTP twin of the inbox pattern)
internal/outbox        Outbox table + the relay that ships it to Kafka
internal/inbox         Consumer-side dedup (processed_events)
internal/event         Kafka wire contracts shared by producer and consumer
internal/gateway       HTTP client for the payment provider
internal/fakegateway   The stand-in provider: idempotent charges + manual settle
internal/webhook       HMAC signing, shared by the sender and the receiver
internal/retry         Bounded backoff with jitter
internal/migrations    Embedded SQL migrations, applied at startup
internal/infra         Postgres and Kafka clients
```
