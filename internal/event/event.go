// Package event holds the Kafka wire contracts shared between services.
// Both the producer (payment-service) and the consumer (payment-worker)
// import this package so they never drift out of sync on topic name or shape.
package event

// HeaderEventID is the Kafka record header carrying the unique event id — the
// outbox row's id. Consumers read it to deduplicate redelivered messages
// (the inbox / processed-events pattern).
//
// The id comes from the stored outbox row, not from the moment of publishing.
// That matters: a relay that generated a fresh id per attempt would make every
// retry look like a brand new event, and consumer dedup would never fire.
const HeaderEventID = "event_id"
