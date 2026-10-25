ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_currency_check;

ALTER TABLE payments
    ALTER COLUMN currency DROP DEFAULT;
