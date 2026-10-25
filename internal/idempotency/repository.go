// Package idempotency records which client requests have already been served.
// It is the HTTP twin of the inbox pattern: the inbox answers "did I already
// handle this event?", while an idempotency key answers "which resource did
// this request already create?" — because unlike an event consumer, an HTTP
// client is waiting for an answer.
//
// The record is written in the same transaction as the resource it points at,
// so the two commit together. A payment can never exist without its key, and
// a key can never point at a payment that was rolled back.
package idempotency

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
)

// ErrNotFound means no row matched the key.
var ErrNotFound = errors.New("idempotency key not found")

// Key is the idempotency_keys table row.
type Key struct {
	Key         string
	RequestHash string
	PaymentID   string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// Repository is the data-access layer for idempotency keys.
type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a Repository whose statements run on tx, so the key row
// commits together with the payment it points at.
func (r *Repository) WithTx(tx *sql.Tx) *Repository {
	return &Repository{db: tx}
}

// Claim tries to record key as taken. It reports whether this caller won the
// key: false means another request already holds it.
//
// ON CONFLICT DO NOTHING is what makes this safe under concurrency. Reading
// the key first and inserting second would race: two requests could both find
// nothing and both try to insert. Here the primary-key index decides the
// winner, and the loser learns it lost without an error.
func (r *Repository) Claim(ctx context.Context, k *Key) (bool, error) {
	const query = `
		INSERT INTO idempotency_keys (key, request_hash, payment_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO NOTHING
		RETURNING created_at, expires_at
	`
	err := r.db.QueryRowContext(ctx, query, k.Key, k.RequestHash, k.PaymentID).
		Scan(&k.CreatedAt, &k.ExpiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim idempotency key: %w", err)
	}
	return true, nil
}

// Get reads the key that won.
func (r *Repository) Get(ctx context.Context, key string) (*Key, error) {
	const query = `
		SELECT key, request_hash, payment_id, created_at, expires_at
		FROM idempotency_keys
		WHERE key = $1
	`
	var k Key
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&k.Key, &k.RequestHash, &k.PaymentID, &k.CreatedAt, &k.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}
	return &k, nil
}
