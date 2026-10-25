package payment

import (
	"context"
	"encoding/json"
	"log"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/muammarahlnn/design-payment-system/internal/event"
)

// DLQPublisher is the slice of the Kafka producer the consumer needs to
// dead-letter a record it cannot handle.
type DLQPublisher interface {
	Publish(ctx context.Context, topic string, key, value []byte, headers map[string]string) error
}

// Consumer reads payment.created and hands each event to the Processor.
// Records it cannot handle go to payment.created.dlq.
type Consumer struct {
	client    *kgo.Client
	dlq       DLQPublisher
	processor *Processor
}

func NewConsumer(client *kgo.Client, dlq DLQPublisher, processor *Processor) *Consumer {
	return &Consumer{client: client, dlq: dlq, processor: processor}
}

// Run polls for records until ctx is cancelled. Start it in its own goroutine.
//
// Offsets are auto-committed by the client (franz-go's default): once handle()
// returns — whether it processed, skipped a duplicate, or dead-lettered — the
// record is considered done and its offset will be committed.
func (c *Consumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		fetches.EachError(func(topic string, partition int32, err error) {
			log.Printf("payment-worker: fetch error on %s[%d]: %v", topic, partition, err)
		})

		fetches.EachRecord(func(record *kgo.Record) {
			c.handle(ctx, record)
		})
	}
}

func (c *Consumer) handle(ctx context.Context, record *kgo.Record) {
	var e event.PaymentCreated
	if err := json.Unmarshal(record.Value, &e); err != nil {
		// A malformed payload will never decode — dead-letter it immediately,
		// no point retrying.
		log.Printf("payment-worker: undecodable %s; dead-lettering: %v", event.TopicPaymentCreated, err)
		c.deadLetter(ctx, record, err)
		return
	}

	eventID := headerValue(record, event.HeaderEventID)
	if eventID == "" {
		log.Printf("payment-worker: %s for payment %s has no %s header; processing without dedup",
			event.TopicPaymentCreated, e.PaymentID, event.HeaderEventID)
	}

	// The processor owns its own retry around the gateway call. Anything that
	// still fails here is either a bad event or a dependency that is down, and
	// both belong in the DLQ rather than blocking the partition forever.
	if err := c.processor.HandlePaymentCreated(ctx, eventID, e); err != nil {
		log.Printf("payment-worker: processing %s for payment %s failed; dead-lettering: %v",
			event.TopicPaymentCreated, e.PaymentID, err)
		c.deadLetter(ctx, record, err)
	}
}

func (c *Consumer) deadLetter(ctx context.Context, record *kgo.Record, cause error) {
	headers := map[string]string{
		"dlq_reason":        cause.Error(),
		"dlq_source":        record.Topic,
		event.HeaderEventID: headerValue(record, event.HeaderEventID),
	}
	if err := c.dlq.Publish(ctx, event.TopicPaymentCreatedDLQ, record.Key, record.Value, headers); err != nil {
		log.Printf("payment-worker: FAILED to dead-letter to %s: %v", event.TopicPaymentCreatedDLQ, err)
	}
}

func headerValue(record *kgo.Record, key string) string {
	for _, h := range record.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
