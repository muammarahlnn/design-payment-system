package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
)

// Status values a payment row may hold. They match the CHECK constraint in
// the migration, so an unknown value fails at the database, not silently.
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusSucceeded  = "SUCCEEDED"
	StatusFailed     = "FAILED"

	// StatusUnknown means the gateway never answered, so we do not know
	// whether the money moved. It is a waiting room: reconciliation asks the
	// gateway what happened and moves it to SUCCEEDED, FAILED, or back to
	// PENDING. Never treat it as "not charged".
	StatusUnknown = "UNKNOWN"
)

// CurrencyIDR is the only currency this project supports. IDR is treated as
// zero-decimal: Payment.Amount counts whole rupiah, not sen.
const CurrencyIDR = "IDR"

// ErrNotFound means no payment row matched the lookup.
var ErrNotFound = errors.New("payment not found")

// Payment is the payments table row. Amount is whole rupiah (see CurrencyIDR).
//
// GatewayReference is the provider's id for the charge, empty until the worker
// has submitted it.
//
// Idempotency is deliberately not here: it lives in internal/idempotency,
// because a key is only useful for the client's retry window while a payment
// is kept for years.
type Payment struct {
	ID               string
	CustomerID       string
	Amount           int64
	Currency         string
	PaymentMethod    string
	Status           string
	GatewayReference string
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Repository is the data-access layer of the payment feature.
type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

// WithTx returns a Repository whose statements run on tx, so the payment
// insert shares a transaction with the idempotency key (and, later, the
// outbox row).
func (r *Repository) WithTx(tx *sql.Tx) *Repository {
	return &Repository{db: tx}
}

// Insert writes a new payment and fills in the fields the database generates
// (id, status default, timestamps).
//
// This is a plain insert with no conflict handling. Two concurrent requests
// carrying the same idempotency key will both reach it and both succeed —
// the idempotency_keys table is what rejects the loser, and its rollback
// removes this row again.
func (r *Repository) Insert(ctx context.Context, p *Payment) error {
	const query = `
		INSERT INTO payments (customer_id, amount, currency, payment_method)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		p.CustomerID, p.Amount, p.Currency, p.PaymentMethod,
	).Scan(&p.ID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

// GetByID reads one payment.
func (r *Repository) GetByID(ctx context.Context, id string) (*Payment, error) {
	const query = `
		SELECT id, customer_id, amount, currency, payment_method,
		       status, COALESCE(gateway_reference, ''), COALESCE(last_error, ''),
		       created_at, updated_at
		FROM payments
		WHERE id = $1
	`
	var p Payment
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.CustomerID, &p.Amount, &p.Currency, &p.PaymentMethod,
		&p.Status, &p.GatewayReference, &p.LastError, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get payment by id: %w", err)
	}
	return &p, nil
}

// UpdateStatus moves a payment from one status to another and records the
// gateway reference. It reports whether a row changed: false means the payment
// was not in the "from" status any more.
//
// The `AND status = $2` is the guard: it makes the transition a compare-and-set
// rather than a blind write. Two workers racing on the same payment cannot both
// succeed, and a redelivered event cannot move a payment that has already moved
// on. This is cheaper than SELECT ... FOR UPDATE because the check and the
// write are one statement.
func (r *Repository) UpdateStatus(ctx context.Context, id, from, to, gatewayRef, lastErr string) (bool, error) {
	const query = `
		UPDATE payments
		SET status = $3,
		    gateway_reference = NULLIF($4, ''),
		    last_error = NULLIF($5, ''),
		    updated_at = now()
		WHERE id = $1 AND status = $2
	`
	res, err := r.db.ExecContext(ctx, query, id, from, to, gatewayRef, lastErr)
	if err != nil {
		return false, fmt.Errorf("update payment status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n == 1, nil
}

// IsTerminal reports whether a status is a final outcome that nothing should
// move a payment out of.
func IsTerminal(status string) bool {
	return status == StatusSucceeded || status == StatusFailed
}

// SettleStatus moves a payment to a terminal status, but only from a
// non-terminal one. It reports whether a row changed.
//
// This is the state-machine guard that makes webhook delivery order stop
// mattering. A payment may legitimately be settled from PENDING, PROCESSING,
// or UNKNOWN — the last of those is the whole point, since a webhook is how an
// UNKNOWN payment learns what really happened. What must never happen is a
// second webhook moving a payment that is already SUCCEEDED or FAILED: the
// first terminal answer wins, and a late or duplicate delivery is ignored.
//
// Doing it in the WHERE clause rather than a read-then-write means two
// concurrent deliveries cannot both succeed, with no lock and no transaction
// of its own.
func (r *Repository) SettleStatus(ctx context.Context, id, to, gatewayRef string) (bool, error) {
	const query = `
		UPDATE payments
		SET status = $2,
		    gateway_reference = COALESCE(NULLIF($3, ''), gateway_reference),
		    updated_at = now()
		WHERE id = $1
		  AND status NOT IN ('SUCCEEDED', 'FAILED')
	`
	res, err := r.db.ExecContext(ctx, query, id, to, gatewayRef)
	if err != nil {
		return false, fmt.Errorf("settle payment status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n == 1, nil
}

// FindStale returns payments that have sat in a non-final status longer than
// olderThan, oldest first. These are the rows reconciliation inspects.
//
// It reads UNKNOWN and PROCESSING only. PENDING is deliberately excluded: a
// PENDING payment has not been handed to the gateway yet, so there is nothing
// to ask about — if it is stuck, the event was lost, which is a different
// problem with a different fix.
func (r *Repository) FindStale(ctx context.Context, olderThan time.Duration, limit int) ([]Payment, error) {
	const query = `
		SELECT id, customer_id, amount, currency, payment_method,
		       status, COALESCE(gateway_reference, ''), COALESCE(last_error, ''),
		       created_at, updated_at
		FROM payments
		WHERE status IN ('UNKNOWN', 'PROCESSING')
		  AND updated_at < now() - $1::interval
		ORDER BY updated_at
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, olderThan.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("find stale payments: %w", err)
	}
	defer rows.Close()

	var out []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(
			&p.ID, &p.CustomerID, &p.Amount, &p.Currency, &p.PaymentMethod,
			&p.Status, &p.GatewayReference, &p.LastError, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale payment: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Touch bumps updated_at without changing status, so a payment the reconciler
// inspected and left alone moves to the back of the queue instead of being
// re-examined every tick.
func (r *Repository) Touch(ctx context.Context, id string) error {
	const query = `UPDATE payments SET updated_at = now() WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("touch payment: %w", err)
	}
	return nil
}
