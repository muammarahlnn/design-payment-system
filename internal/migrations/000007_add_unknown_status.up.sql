-- Add UNKNOWN: the gateway never answered, so the charge may or may not have
-- happened.
--
-- Before this, a timeout left the payment in PENDING. That was a lie. PENDING
-- means "not submitted yet", and anything reading it — a person, a retry job,
-- a dashboard — would conclude it is safe to charge. After a timeout it is not.
--
-- UNKNOWN is a waiting room, not a new branch of the lifecycle. Reconciliation
-- asks the gateway what really happened and drains it:
--
--   PENDING  --accepted--> PROCESSING --webhook--> SUCCEEDED | FAILED
--   PENDING  --declined--> FAILED
--   PENDING  --no answer-> UNKNOWN   --reconcile-> SUCCEEDED | FAILED | PENDING
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;

ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('PENDING', 'PROCESSING', 'UNKNOWN', 'SUCCEEDED', 'FAILED'));

-- Why the charge ended where it did. For UNKNOWN this is what reconciliation
-- and a support agent read first: "timed out" and "gateway returned 500 three
-- times" need different follow-up.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS last_error TEXT;

-- Reconciliation scans UNKNOWN oldest-first, so index exactly those rows.
-- This replaces the PROCESSING + NULL-reference index from migration 6, which
-- encoded "we never heard back" implicitly; UNKNOWN now says it outright.
DROP INDEX IF EXISTS idx_payments_processing_unreferenced;

CREATE INDEX IF NOT EXISTS idx_payments_unknown
    ON payments (updated_at)
    WHERE status = 'UNKNOWN';
