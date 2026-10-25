DROP INDEX IF EXISTS idx_payments_unknown;

-- Nothing can stay UNKNOWN once the constraint no longer allows it.
UPDATE payments SET status = 'PENDING' WHERE status = 'UNKNOWN';

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCEEDED', 'FAILED'));

ALTER TABLE payments DROP COLUMN IF EXISTS last_error;

CREATE INDEX IF NOT EXISTS idx_payments_processing_unreferenced
    ON payments (updated_at)
    WHERE status = 'PROCESSING' AND gateway_reference IS NULL;
