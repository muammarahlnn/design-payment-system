package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/muammarahlnn/design-payment-system/internal/event"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
)

// Publisher is the transport the relay ships messages onto. The Kafka
// implementation is kafka.Producer.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte, headers map[string]string) error
}

const (
	defaultPollInterval = 1 * time.Second
	defaultBatchSize    = 100
	defaultFlushTimeout = 30 * time.Second
)

// Relay is the "message relay" half of the transactional outbox pattern:
// business code writes outbox rows in its own transaction, and this loop ships
// them to Kafka. Delivery is at-least-once — a row already sent to Kafka can be
// sent again after a crash, so consumers must be idempotent.
//
// Running more than one relay: FOR UPDATE SKIP LOCKED means two relays never
// claim the same row — they split the backlog, they do not duplicate it. The
// real cost of one relay per service replica is ORDERING. If relay A holds the
// older batch and relay B the newer one, B can reach Kafka first, and two
// events for the same payment land out of order on their partition. The cheap
// fix is leader election (one relay at a time); a dedicated relay deployment
// is the next step. The other cost is simply N× the poll load on Postgres.
//
// Duplicates come from somewhere else, and a single relay has them too: a
// publish that succeeds followed by a transaction that fails to commit leaves
// the row unpublished, so the next tick ships it again. That is the
// at-least-once guarantee, and it is why consumers must deduplicate.
type Relay struct {
	db        *sql.DB
	repo      *Repository
	publisher Publisher

	pollInterval time.Duration
	batchSize    int
	flushTimeout time.Duration
}

func NewRelay(db *sql.DB, repo *Repository, publisher Publisher) *Relay {
	return &Relay{
		db:           db,
		repo:         repo,
		publisher:    publisher,
		pollInterval: defaultPollInterval,
		batchSize:    defaultBatchSize,
		flushTimeout: defaultFlushTimeout,
	}
}

// Run polls until ctx is cancelled. Start it in its own goroutine.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			flushCtx, cancel := context.WithTimeout(ctx, r.flushTimeout)
			n, err := r.flush(flushCtx)
			cancel()

			switch {
			case err != nil:
				log.Printf("outbox: flush failed: %v", err)
			case n > 0:
				log.Printf("outbox: published %d message(s)", n)
			}
		}
	}
}

// flush ships one batch inside a single transaction: rows are locked, each is
// published, then all are stamped published_at — commit ties it together. If
// any publish fails the whole transaction rolls back and the batch is retried
// next tick; rows already sent to Kafka in that attempt get re-sent
// (at-least-once). Ordering is preserved because the batch never advances past
// a failure.
func (r *Relay) flush(ctx context.Context) (int, error) {
	var published int

	err := postgres.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		repo := r.repo.WithTx(tx)

		messages, err := repo.FetchUnpublishedForUpdate(ctx, r.batchSize)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return nil
		}

		ids := make([]string, 0, len(messages))
		for i := range messages {
			m := &messages[i]

			// The event id travels as a header and comes from the stored row,
			// so a redelivery carries the same id and the consumer can dedup.
			// The partition key is the payment id, so every event for one
			// payment lands on one partition and stays in order.
			headers := map[string]string{event.HeaderEventID: m.ID}
			if err := r.publisher.Publish(ctx, m.EventType, []byte(m.AggregateID), m.Payload, headers); err != nil {
				return fmt.Errorf("publish message %s: %w", m.ID, err)
			}
			ids = append(ids, m.ID)
		}

		if err := repo.MarkPublished(ctx, ids); err != nil {
			return err
		}
		published = len(ids)
		return nil
	})

	return published, err
}
