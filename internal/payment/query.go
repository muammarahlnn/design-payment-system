package payment

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ListItem is a payment as the UI lists it.
type ListItem struct {
	ID               string    `json:"id"`
	CustomerID       string    `json:"customer_id"`
	Amount           int64     `json:"amount"`
	Currency         string    `json:"currency"`
	PaymentMethod    string    `json:"payment_method"`
	Status           string    `json:"status"`
	GatewayReference string    `json:"gateway_reference"`
	LastError        string    `json:"last_error"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TimelineStep is one observable moment in a payment's life. The UI draws
// these in order so the whole pipeline is visible for a single payment.
type TimelineStep struct {
	Stage  string     `json:"stage"`
	Detail string     `json:"detail"`
	At     *time.Time `json:"at"`
	Done   bool       `json:"done"`
}

// Detail is one payment plus everything we know about how it got there.
type Detail struct {
	Payment  ListItem       `json:"payment"`
	Timeline []TimelineStep `json:"timeline"`
}

// List returns the most recent payments.
func (r *Repository) List(ctx context.Context, limit int, status string) ([]ListItem, error) {
	const query = `
		SELECT id, customer_id, amount, currency, payment_method, status,
		       COALESCE(gateway_reference, ''), COALESCE(last_error, ''),
		       created_at, updated_at
		FROM payments
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	items := make([]ListItem, 0, limit)
	for rows.Next() {
		var it ListItem
		if err := rows.Scan(
			&it.ID, &it.CustomerID, &it.Amount, &it.Currency, &it.PaymentMethod,
			&it.Status, &it.GatewayReference, &it.LastError, &it.CreatedAt, &it.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// Counts returns how many payments sit in each status, for the dashboard.
func (r *Repository) Counts(ctx context.Context) (map[string]int, error) {
	const query = `SELECT status, count(*) FROM payments GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count payments: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// GetDetail builds the timeline for one payment by reading the traces each
// stage leaves behind: the payments row, its outbox message, and the inbox
// marker the worker wrote when it handled the event.
//
// Nothing here is a dedicated audit log — every timestamp is a side effect of
// a mechanism that exists for correctness. That is the point worth seeing.
func (r *Repository) GetDetail(ctx context.Context, id string) (*Detail, error) {
	const payQuery = `
		SELECT id, customer_id, amount, currency, payment_method, status,
		       COALESCE(gateway_reference, ''), COALESCE(last_error, ''),
		       created_at, updated_at
		FROM payments WHERE id = $1
	`
	var p ListItem
	err := r.db.QueryRowContext(ctx, payQuery, id).Scan(
		&p.ID, &p.CustomerID, &p.Amount, &p.Currency, &p.PaymentMethod,
		&p.Status, &p.GatewayReference, &p.LastError, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get payment detail: %w", err)
	}

	const traceQuery = `
		SELECT o.id, o.created_at, o.published_at, pe.processed_at
		FROM outbox o
		LEFT JOIN processed_events pe
		       ON pe.event_id = o.id::text AND pe.consumer = $2
		WHERE o.aggregate_id = $1
		ORDER BY o.created_at
		LIMIT 1
	`
	var (
		outboxID    sql.NullString
		queuedAt    sql.NullTime
		publishedAt sql.NullTime
		processedAt sql.NullTime
	)
	err = r.db.QueryRowContext(ctx, traceQuery, id, ConsumerName).
		Scan(&outboxID, &queuedAt, &publishedAt, &processedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get payment trace: %w", err)
	}

	tl := []TimelineStep{
		{Stage: "Payment created", Detail: "status PENDING, written with its idempotency key", At: &p.CreatedAt, Done: true},
	}

	add := func(stage, detail string, t sql.NullTime) {
		step := TimelineStep{Stage: stage, Detail: detail}
		if t.Valid {
			at := t.Time
			step.At, step.Done = &at, true
		}
		tl = append(tl, step)
	}

	add("Queued in outbox", "payment.created event, same transaction as the payment", queuedAt)
	add("Published to Kafka", "relay shipped it to topic payment.created", publishedAt)
	add("Worker charged the gateway", "consumed the event and submitted the charge", processedAt)

	settled := TimelineStep{Stage: "Settled", Detail: "terminal status reached"}
	switch p.Status {
	case StatusSucceeded, StatusFailed:
		at := p.UpdatedAt
		settled.At, settled.Done = &at, true
		settled.Detail = "settled as " + p.Status
	case StatusUnknown:
		settled.Detail = "gateway never answered — waiting on reconciliation"
	default:
		settled.Detail = "waiting for the customer to pay"
	}
	tl = append(tl, settled)

	return &Detail{Payment: p, Timeline: tl}, nil
}
