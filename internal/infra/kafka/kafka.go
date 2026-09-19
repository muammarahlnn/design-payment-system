package kafka

import (
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/muammarahlnn/design-payment-system/internal/config"
)

// New builds a franz-go client from the injected seed brokers, plus any extra
// client options the caller needs (e.g. kgo.ConsumerGroup + kgo.ConsumeTopics
// for a consumer). No connection is established until the client is first
// used, and a plain producer client just passes no extra opts.
func New(cfg config.KafkaConfig, opts ...kgo.Opt) (*kgo.Client, error) {
	clientOpts := append([]kgo.Opt{kgo.SeedBrokers(cfg.Brokers...)}, opts...)
	return kgo.NewClient(clientOpts...)
}
