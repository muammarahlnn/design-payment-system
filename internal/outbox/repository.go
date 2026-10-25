// Package outbox implements the transactional outbox pattern. Business code
// writes an outbox row inside its own transaction, and the Relay ships those
// rows to Kafka afterwards.
//
// It exists because a database commit and a Kafka publish cannot be made
// atomic. Publishing directly leaves two failure modes: publish-then-crash
// before commit (an event for a payment that does not exist), or
// commit-then-crash before publish (a payment nobody hears about). Writing
// both rows to one database removes the choice.
package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
)

// Message is one row of the outbox table: a domain event committed alongside
// its business data and waiting to be shipped to Kafka.
type Message struct {
	ID            string
	AggregateType string // e.g. "payment"
	AggregateID   string // the payment id — also used as the Kafka partition key
	EventType     string // e.g. "payment.created" — the destination topic
	Payload       []byte // the serialized event (JSON)
	CreatedAt     time.Time
	PublishedAt   *time.Time // nil until the relay has shipped it
}

// Repository is the data-access layer for the outbox table.
type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a Repository whose statements run on tx. Callers writing an
// outbox row alongside business data use this so both land in one transaction.
func (r *Repository) WithTx(tx *sql.Tx) *Repository {
	return &Repository{db: tx}
}

// Insert appends a message to the outbox. Run it via WithTx(tx) so the message
// is committed in the same transaction as the business row.
func (r *Repository) Insert(ctx context.Context, m *Message) error {
	const query = `
		INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		m.AggregateType, m.AggregateID, m.EventType, m.Payload,
	).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert outbox message: %w", err)
	}
	return nil
}

// FetchUnpublishedForUpdate returns up to limit unshipped messages, oldest
// first, locking them with FOR UPDATE SKIP LOCKED so that concurrent relay
// runs (or multiple service replicas) never pick up the same rows. Must be
// called inside a transaction — the locks are held until it commits.
//
// This is the case where SELECT ... FOR UPDATE is the right tool and
// ON CONFLICT is not: the rows already exist, and the job is to claim them.
func (r *Repository) FetchUnpublishedForUpdate(ctx context.Context, limit int) ([]Message, error) {
	const query = `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query unpublished: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.AggregateType, &m.AggregateID, &m.EventType, &m.Payload, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox row: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// MarkPublished stamps published_at = now() on the given message ids.
func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	const query = `UPDATE outbox SET published_at = now() WHERE id = ANY($1::uuid[])`
	if _, err := r.db.ExecContext(ctx, query, pq.Array(ids)); err != nil {
		return fmt.Errorf("mark published: %w", err)
	}
	return nil
}
