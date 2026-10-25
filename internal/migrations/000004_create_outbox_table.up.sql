-- The transactional outbox. Business code writes a row here in the SAME
-- transaction as the payment it describes, so the two commit together. A
-- payment can never exist without its event queued, and an event can never
-- describe a payment that was rolled back.
--
-- This is what you cannot get by publishing to Kafka directly: a database
-- commit and a Kafka publish are two systems, and nothing makes them atomic.
-- Writing both rows to one database does make them atomic, and a separate
-- relay ships the queued row afterwards.
CREATE TABLE IF NOT EXISTS outbox (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,        -- "payment"
    aggregate_id   UUID NOT NULL,        -- the payment id; also the Kafka partition key
    event_type     TEXT NOT NULL,        -- "payment.created" — also the topic
    payload        JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ           -- NULL until the relay has shipped it
);

-- The relay only ever scans for not-yet-published rows, oldest first. A partial
-- index keeps that query fast and the index itself tiny — it holds only the
-- unshipped backlog, not the whole (ever-growing) table.
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished
    ON outbox (created_at, id)
    WHERE published_at IS NULL;
