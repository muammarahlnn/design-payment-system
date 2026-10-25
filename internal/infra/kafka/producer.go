package kafka

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer is a thin, topic-agnostic wrapper around *kgo.Client for
// synchronous publishing. It exists so callers (like the outbox relay) depend
// on a small Publish method rather than the whole Kafka client.
type Producer struct {
	client *kgo.Client
}

func NewProducer(client *kgo.Client) *Producer {
	return &Producer{client: client}
}

// Publish produces one record and blocks until the broker acknowledges it
// (or ctx is cancelled). With franz-go's defaults that ack means acks=all +
// idempotent write, which is what stops an in-flight retry from reordering
// records inside a partition.
//
// key sets the partition: Kafka hashes it, so every record sharing a key
// lands on one partition and stays ordered relative to the others.
func (p *Producer) Publish(ctx context.Context, topic string, key, value []byte, headers map[string]string) error {
	record := &kgo.Record{Topic: topic, Key: key, Value: value}
	for k, v := range headers {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	return p.client.ProduceSync(ctx, record).FirstErr()
}
