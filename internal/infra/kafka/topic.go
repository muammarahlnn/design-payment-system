package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

// EnsureTopic creates topic if it does not already exist. It is idempotent:
// an existing topic (even with different settings) is treated as success.
//
// franz-go producers do not auto-create topics, so the owner of an event
// stream calls this once at startup — the Kafka equivalent of running a
// migration before touching a table.
func EnsureTopic(ctx context.Context, client *kgo.Client, topic string, partitions int32, replication int16) error {
	admin := kadm.NewClient(client)

	resp, err := admin.CreateTopic(ctx, partitions, replication, nil, topic)
	if err != nil && !errors.Is(err, kerr.TopicAlreadyExists) {
		return fmt.Errorf("create topic %s: %w", topic, err)
	}
	if resp.Err != nil && !errors.Is(resp.Err, kerr.TopicAlreadyExists) {
		return fmt.Errorf("create topic %s: %w", topic, resp.Err)
	}
	return nil
}
