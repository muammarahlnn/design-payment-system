CREATE TABLE IF NOT EXISTS payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- The client's idempotency key. UNIQUE is what actually enforces
    -- idempotency: two concurrent requests with the same key cannot both
    -- insert, because Postgres rejects the second one. The application never
    -- has to check-then-insert, which would race.
    idempotency_key TEXT NOT NULL UNIQUE,

    -- SHA-256 of the request fields. A client that reuses one key with a
    -- different body has a bug, and silently returning the first payment
    -- would hide it. Storing the hash lets us reject that case.
    request_hash    TEXT NOT NULL,

    customer_id     TEXT NOT NULL,

    -- Money is stored in minor units (cents) as an integer, never as a float.
    -- NUMERIC would also be exact, but integer minor units match what payment
    -- gateways accept on the wire and remove every rounding decision.
    amount          BIGINT NOT NULL CHECK (amount > 0),
    currency        CHAR(3) NOT NULL,
    payment_method  TEXT NOT NULL,

    status          TEXT NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCEEDED', 'FAILED')),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
