ALTER TABLE payments ADD COLUMN IF NOT EXISTS idempotency_key TEXT;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS request_hash TEXT;

-- Move the keys back onto the payment rows before dropping the table.
UPDATE payments p
SET idempotency_key = k.key,
    request_hash    = k.request_hash
FROM idempotency_keys k
WHERE k.payment_id = p.id;

DROP TABLE IF EXISTS idempotency_keys;
