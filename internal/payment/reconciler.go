package payment

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/muammarahlnn/design-payment-system/internal/gateway"
	"github.com/muammarahlnn/design-payment-system/internal/inbox"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
)

// ReconcilerConsumerName identifies reconciliation in processed_events, so its
// settlements dedup separately from the worker's and the webhook's.
const ReconcilerConsumerName = "payment-reconciler"

// ChargeReader is the query side of the gateway. Declared here because this is
// where it is used.
type ChargeReader interface {
	GetCharge(ctx context.Context, paymentID string) (*gateway.ChargeResult, error)
}

// ReconcilerConfig tunes the sweep.
type ReconcilerConfig struct {
	// Interval is how often to sweep.
	Interval time.Duration
	// StaleAfter is how long a payment may sit in UNKNOWN or PROCESSING
	// before reconciliation looks at it. Too short and it races the normal
	// path; too long and money stays in limbo.
	StaleAfter time.Duration
	// ExpireAfter is how long a payment the gateway still calls ACCEPTED may
	// wait for a customer who never pays, before being written off as FAILED.
	ExpireAfter time.Duration
	// BatchSize caps rows per sweep.
	BatchSize int
}

// Reconciler settles payments the normal path left behind.
//
// Every other part of this system is event-driven, and every event-driven
// system loses events: a callback that never arrives, a worker that dies
// between charging and committing, a gateway that times out. Reconciliation is
// the backstop that does not depend on anything being delivered — it polls our
// own database, asks the provider directly, and repairs the difference.
//
// It is deliberately the only component that treats the gateway as the source
// of truth about money. Our database records what we *believe*; the provider
// knows what actually happened.
type Reconciler struct {
	db       *sql.DB
	payments *Repository
	inbox    *inbox.Repository
	gateway  ChargeReader
	cfg      ReconcilerConfig
}

func NewReconciler(db *sql.DB, payments *Repository, inboxRepo *inbox.Repository, gw ChargeReader, cfg ReconcilerConfig) *Reconciler {
	return &Reconciler{db: db, payments: payments, inbox: inboxRepo, gateway: gw, cfg: cfg}
}

// Run sweeps until ctx is cancelled. Start it in its own goroutine.
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	log.Printf("reconciler: sweeping every %s for payments stuck over %s (expire after %s)",
		r.cfg.Interval, r.cfg.StaleAfter, r.cfg.ExpireAfter)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := r.sweep(ctx); err != nil {
				log.Printf("reconciler: sweep failed: %v", err)
			} else if n > 0 {
				log.Printf("reconciler: inspected %d stuck payment(s)", n)
			}
		}
	}
}

func (r *Reconciler) sweep(ctx context.Context) (int, error) {
	stale, err := r.payments.FindStale(ctx, r.cfg.StaleAfter, r.cfg.BatchSize)
	if err != nil {
		return 0, err
	}

	for i := range stale {
		if err := r.reconcile(ctx, &stale[i]); err != nil {
			// One bad payment must not stop the sweep.
			log.Printf("reconciler: payment %s: %v", stale[i].ID, err)
		}
	}
	return len(stale), nil
}

// reconcile asks the gateway about one payment and acts on the answer.
func (r *Reconciler) reconcile(ctx context.Context, p *Payment) error {
	charge, err := r.gateway.GetCharge(ctx, p.ID)

	switch {
	case errors.Is(err, gateway.ErrNoSuchCharge):
		// The gateway has no record. For a real provider that means the
		// charge never happened, and the tempting move is to fail the payment
		// or send it back to PENDING.
		//
		// We do neither, on purpose. "No record" and "I lost my records" look
		// identical over HTTP, and acting on the second one either loses a
		// real payment or charges a customer twice. A human decides.
		log.Printf("reconciler: payment %s (%s since %s) — gateway has NO RECORD; leaving for manual review",
			p.ID, p.Status, p.UpdatedAt.Format(time.RFC3339))
		return r.payments.Touch(ctx, p.ID)

	case err != nil:
		// Could not reach the gateway. Leave it; the next sweep tries again.
		return err
	}

	switch charge.Status {
	case "SUCCEEDED":
		return r.settle(ctx, p, StatusSucceeded, charge.Reference, "reconciled: gateway reports SUCCEEDED")

	case "FAILED", "DECLINED":
		return r.settle(ctx, p, StatusFailed, charge.Reference, "reconciled: gateway reports "+charge.Status)

	case "ACCEPTED":
		// The gateway has the charge but the customer has not paid.
		//
		// For an UNKNOWN payment this is already valuable: we now know the
		// charge exists, and we have the reference we never received. It
		// becomes an ordinary PROCESSING payment awaiting its webhook.
		if p.Status == StatusUnknown {
			log.Printf("reconciler: payment %s was UNKNOWN; gateway has it as ACCEPTED -> PROCESSING (ref %s)",
				p.ID, charge.Reference)
			return r.promoteToProcessing(ctx, p, charge.Reference)
		}

		// Already PROCESSING. If it has waited longer than anyone sensibly
		// waits for a customer, write it off.
		if time.Since(p.CreatedAt) > r.cfg.ExpireAfter {
			log.Printf("reconciler: payment %s unpaid for %s; expiring",
				p.ID, time.Since(p.CreatedAt).Truncate(time.Second))
			return r.settle(ctx, p, StatusFailed, charge.Reference, "reconciled: expired, customer never paid")
		}

		// Still within the window. Leave it, but move it to the back of the
		// queue so the next sweep looks at older rows first.
		return r.payments.Touch(ctx, p.ID)

	default:
		log.Printf("reconciler: payment %s — gateway reports unrecognized status %q", p.ID, charge.Status)
		return r.payments.Touch(ctx, p.ID)
	}
}

// settle writes a terminal status, recording the decision in the inbox so it
// is traceable and so a repeat sweep is a no-op.
//
// It uses the same SettleStatus guard the webhook uses, which is what makes
// the two safe to run concurrently: whichever commits first wins, and the
// loser's UPDATE matches no rows.
func (r *Reconciler) settle(ctx context.Context, p *Payment, status, reference, note string) error {
	return postgres.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		// One marker per payment per decision, so a sweep that runs twice
		// does not log the same settlement twice.
		newly, err := r.inbox.WithTx(tx).MarkProcessed(ctx, ReconcilerConsumerName, "settle:"+p.ID)
		if err != nil {
			return err
		}
		if !newly {
			return nil
		}

		settled, err := r.payments.WithTx(tx).SettleStatus(ctx, p.ID, status, reference)
		if err != nil {
			return err
		}
		if !settled {
			// The webhook got there first. Exactly what the guard is for.
			log.Printf("reconciler: payment %s already settled by someone else; standing down", p.ID)
			return nil
		}

		log.Printf("reconciler: payment %s %s -> %s (%s)", p.ID, p.Status, status, note)
		return nil
	})
}

// promoteToProcessing moves an UNKNOWN payment to PROCESSING and backfills the
// reference we never received. Not terminal, so it uses the compare-and-set.
func (r *Reconciler) promoteToProcessing(ctx context.Context, p *Payment, reference string) error {
	updated, err := r.payments.UpdateStatus(ctx, p.ID, StatusUnknown, StatusProcessing, reference,
		"reconciled: gateway confirmed the charge exists")
	if err != nil {
		return err
	}
	if !updated {
		log.Printf("reconciler: payment %s moved out of UNKNOWN concurrently; leaving it", p.ID)
	}
	return nil
}
