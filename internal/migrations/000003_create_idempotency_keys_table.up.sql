-- Move idempotency out of the payments table.
--
-- Why: the two have different lifetimes. A payment row is kept for years
-- (audit, accounting, dispute, reconciliation). An idempotency key is only
-- useful for the client's retry window, about 24 hours. As a column on
-- payments, the key could never be deleted without deleting the payment, so
-- the unique index would grow forever.
--
-- Splitting them also means one mechanism serves every endpoint later
-- (/refunds, /payouts), instead of repeating the columns per table.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    -- The key is the primary key, so uniqueness is enforced by the same index
    -- that looks it up. This is what actually makes the API idempotent.
    key          TEXT PRIMARY KEY,

    -- SHA-256 of the request fields. A client that reuses one key with a
    -- different body has a bug; comparing the hash lets us reject that
    -- instead of silently returning the first payment.
    request_hash TEXT NOT NULL,

    payment_id   UUID NOT NULL REFERENCES payments(id),

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Nothing deletes expired rows yet. A cleanup job comes later; this
    -- column is what it will read.
    expires_at   TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '24 hours'
);

-- The cleanup job scans by expiry, so index it.
CREATE INDEX IF NOT EXISTS idempotency_keys_expires_at_idx
    ON idempotency_keys (expires_at);

-- Backfill BEFORE dropping the columns, or the existing keys are lost. This
-- is the step that migration 000002 was missing: never change a constraint or
-- drop a column on a table with rows until the data already fits the new shape.
INSERT INTO idempotency_keys (key, request_hash, payment_id)
SELECT idempotency_key, request_hash, id
FROM payments
ON CONFLICT (key) DO NOTHING;

ALTER TABLE payments DROP COLUMN IF EXISTS idempotency_key;
ALTER TABLE payments DROP COLUMN IF EXISTS request_hash;
