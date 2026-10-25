-- The gateway's own id for this charge, returned when the worker submits it.
-- NULL until the worker has successfully called the gateway.
--
-- It is also the reconciliation handle: a payment sitting in PROCESSING with a
-- NULL reference means the worker never got an answer, and a later job can ask
-- the gateway what actually happened.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway_reference TEXT;

-- Finding payments that are stuck mid-flight is the reconciliation query, so
-- index exactly those rows.
CREATE INDEX IF NOT EXISTS idx_payments_processing_unreferenced
    ON payments (updated_at)
    WHERE status = 'PROCESSING' AND gateway_reference IS NULL;
