-- Only rows whose id actually parses as a UUID can survive the narrowing.
DELETE FROM processed_events
WHERE event_id !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';

ALTER TABLE processed_events ALTER COLUMN event_id TYPE UUID USING event_id::uuid;
