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
| `postgres`        | 5432  | Payment state and the outbox table. |
| `kafka`           | 29092 | Event stream. `kafka:9092` inside the network, `localhost:29092` from the host. |
| `kafka-console`   | 8088  | Redpanda Console. Shows topics, messages, and headers. Dev-only, no auth. |

## Status

Scaffolding only. All six containers build, start, and pass their health
checks. This project does not implement payment logic yet. Each service
creates its config and infrastructure clients, and exposes `/health`.

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
`localhost:5432` and Kafka at `localhost:29092`:

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
internal/infra         Postgres and Kafka clients
```
