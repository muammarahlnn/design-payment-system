DROP INDEX IF EXISTS idx_payments_processing_unreferenced;
ALTER TABLE payments DROP COLUMN IF EXISTS gateway_reference;
