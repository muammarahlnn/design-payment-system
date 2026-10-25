-- Consumer-side idempotency: the inbox pattern. A durable record of which
-- events a consumer has already handled, written in the same transaction as
-- the event's effects so the two commit atomically.
--
-- This is the mirror image of the outbox. The outbox makes sure an event is
-- never lost on the way out; the inbox makes sure it is never applied twice on
-- the way in. Kafka delivery is at-least-once, so both are required.
CREATE TABLE IF NOT EXISTS processed_events (
    consumer     TEXT NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- (consumer, event_id) is both the dedup constraint and the lookup index.
    -- consumer is part of the key so several consumers can share this table
    -- without colliding on the same event id.
    PRIMARY KEY (consumer, event_id)
);
