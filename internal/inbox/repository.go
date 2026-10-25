// Package inbox implements consumer-side idempotency: a durable record of which
// events a consumer has already handled. The marker is written in the same
// transaction as the event's effects, so the two commit atomically — the
// mirror image of the outbox pattern.
package inbox

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
)

// Repository is the data-access layer for the processed_events table.
type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a Repository whose statements run on tx, so the processed
// marker commits together with the event's effects.
func (r *Repository) WithTx(tx *sql.Tx) *Repository {
	return &Repository{db: tx}
}

// MarkProcessed records that consumer has handled eventID. It reports whether
// the marker was newly recorded: false means that pair was already present, so
// the event is a redelivery whose effects are already committed and the caller
// should skip it.
func (r *Repository) MarkProcessed(ctx context.Context, consumer, eventID string) (bool, error) {
	const query = `
		INSERT INTO processed_events (consumer, event_id)
		VALUES ($1, $2)
		ON CONFLICT (consumer, event_id) DO NOTHING
	`
	res, err := r.db.ExecContext(ctx, query, consumer, eventID)
	if err != nil {
		return false, fmt.Errorf("mark processed: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n == 1, nil
}

// IsProcessed is a read-only fast path, used to skip work before doing it.
// It is only an optimization: correctness comes from MarkProcessed, which runs
// inside the same transaction as the effects.
func (r *Repository) IsProcessed(ctx context.Context, consumer, eventID string) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM processed_events WHERE consumer = $1 AND event_id = $2)`
	var exists bool
	if err := r.db.QueryRowContext(ctx, query, consumer, eventID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check processed: %w", err)
	}
	return exists, nil
}
