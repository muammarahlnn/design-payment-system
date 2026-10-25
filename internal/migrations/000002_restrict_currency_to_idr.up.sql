-- This project handles IDR only (see README). Two consequences:
--
-- 1. IDR is a zero-decimal currency in practice. ISO 4217 gives it exponent 2
--    (1 rupiah = 100 sen), but sen is obsolete and Indonesian gateways reject
--    decimals. Midtrans requires whole rupiah as an integer. So `amount` now
--    holds WHOLE RUPIAH, not cents: 1050 means Rp 1.050, not Rp 10,50.
--
-- 2. BIGINT matters more here than in USD. INT tops out at 2,147,483,647,
--    which is about Rp 2,1 miliar (~USD 130k) — a normal property or B2B
--    transfer would overflow it. BIGINT holds far more than Indonesia's GDP.
--
-- The CHECK makes the assumption explicit. To support more currencies later,
-- drop this constraint and add a currency table carrying each one's exponent.
ALTER TABLE payments
    ALTER COLUMN currency SET DEFAULT 'IDR';

ALTER TABLE payments
    ADD CONSTRAINT payments_currency_check CHECK (currency = 'IDR');
