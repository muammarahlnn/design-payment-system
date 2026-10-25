package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/muammarahlnn/design-payment-system/internal/event"
	"github.com/muammarahlnn/design-payment-system/internal/gateway"
	"github.com/muammarahlnn/design-payment-system/internal/inbox"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
	"github.com/muammarahlnn/design-payment-system/internal/retry"
)

// ConsumerName identifies payment-worker in the processed_events table.
const ConsumerName = "payment-worker"

// GatewayClient is the slice of the payment provider the processor needs.
// Declared here, where it is used, so this package does not depend on the
// gateway implementation and can be tested with a stub.
type GatewayClient interface {
	Charge(ctx context.Context, req gateway.ChargeRequest) (*gateway.ChargeResult, error)
}

// Processor is the worker-side half of the payment feature: it takes a
// payment.created event, submits the charge, and records the outcome.
type Processor struct {
	db       *sql.DB
	payments *Repository
	inbox    *inbox.Repository
	gateway  GatewayClient
	policy   retry.Policy
}

func NewProcessor(db *sql.DB, payments *Repository, inboxRepo *inbox.Repository, gw GatewayClient) *Processor {
	return &Processor{
		db:       db,
		payments: payments,
		inbox:    inboxRepo,
		gateway:  gw,
		policy: retry.Policy{
			MaxAttempts: 3,
			BaseDelay:   500 * time.Millisecond,
			MaxDelay:    5 * time.Second,
			Multiplier:  2,
		},
	}
}

// HandlePaymentCreated submits the payment to the gateway and moves it out of
// PENDING.
//
// The order of operations is the important part, and it is deliberate:
//
//  1. skip if already processed, or no longer PENDING
//  2. call the gateway  ← external, OUTSIDE any transaction
//  3. record the result ← one transaction: inbox marker + status
//
// Calling the gateway before writing anything means a crash can only ever
// leave "charged but not recorded", which the redelivered event repairs: the
// charge is keyed by the payment id, so the second attempt replays the first
// answer instead of taking the money again. The opposite order would leave
// "recorded but not charged", and no retry fixes that — the inbox marker
// already says the event is done.
//
// Holding a database transaction open across an HTTP call is also how you
// exhaust a connection pool when the provider gets slow.
func (p *Processor) HandlePaymentCreated(ctx context.Context, eventID string, e event.PaymentCreated) error {
	if eventID != "" {
		// Read-only fast path. Correctness does not rest on it — the real
		// guard is MarkProcessed inside the transaction below.
		processed, err := p.inbox.IsProcessed(ctx, ConsumerName, eventID)
		if err != nil {
			return err
		}
		if processed {
			log.Printf("payment-worker: skipping already-processed event %s (payment %s)", eventID, e.PaymentID)
			return nil
		}
	}

	pay, err := p.payments.GetByID(ctx, e.PaymentID)
	if err != nil {
		return fmt.Errorf("load payment %s: %w", e.PaymentID, err)
	}
	if pay.Status != StatusPending {
		log.Printf("payment-worker: payment %s is already %s, nothing to do", pay.ID, pay.Status)
		return nil
	}

	outcome := p.charge(ctx, pay)
	if outcome.fatal != nil {
		// We could not even form a verdict — the event itself is the problem.
		return outcome.fatal
	}

	return postgres.RunInTx(ctx, p.db, func(tx *sql.Tx) error {
		if eventID != "" {
			newly, err := p.inbox.WithTx(tx).MarkProcessed(ctx, ConsumerName, eventID)
			if err != nil {
				return err
			}
			if !newly {
				// Another delivery of this event committed while we were
				// talking to the gateway. Its effects are already applied.
				log.Printf("payment-worker: event %s was processed concurrently; skipping", eventID)
				return nil
			}
		}

		updated, err := p.payments.WithTx(tx).UpdateStatus(
			ctx, pay.ID, StatusPending, outcome.status, outcome.reference, outcome.lastError)
		if err != nil {
			return err
		}
		if !updated {
			// The payment left PENDING between our read and this write. Not
			// an error: the compare-and-set did its job.
			log.Printf("payment-worker: payment %s moved out of PENDING concurrently; not overwriting", pay.ID)
			return nil
		}

		log.Printf("payment-worker: payment %s -> %s (gateway ref %q, note %q)",
			pay.ID, outcome.status, outcome.reference, outcome.lastError)
		return nil
	})
}

// chargeOutcome is the verdict for one payment: which status it should move to,
// and why. fatal is set only when we could not reach a verdict at all.
type chargeOutcome struct {
	status    string
	reference string
	lastError string
	fatal     error
}

// charge submits the payment and decides what the result means.
//
// Three verdicts, and the difference between them is the whole of step 5:
//
//	accepted       -> PROCESSING   the gateway has it; the webhook closes it
//	declined       -> FAILED       a refusal is an answer. Do not retry.
//	no answer      -> UNKNOWN      we do not know if the money moved.
//
// Retries happen only for failures we know did not charge — connection
// refused, DNS, 5xx. A timeout is never retried inline: see
// gateway.ErrIndeterminate for why. Exhausting the retry budget also lands in
// UNKNOWN rather than leaving the payment in PENDING, because after several
// failed attempts we no longer have grounds to claim nothing happened.
func (p *Processor) charge(ctx context.Context, pay *Payment) chargeOutcome {
	req := gateway.ChargeRequest{
		// The payment id is the idempotency key. It is stable across every
		// retry and every redelivery, which is what makes charging twice
		// impossible on the provider's side.
		IdempotencyKey: pay.ID,
		Amount:         pay.Amount,
		Currency:       pay.Currency,
		PaymentMethod:  pay.PaymentMethod,
	}

	var out chargeOutcome
	retryErr := retry.Do(ctx, p.policy, func() error {
		res, err := p.gateway.Charge(ctx, req)

		switch {
		case err == nil:
			out = chargeOutcome{status: StatusProcessing, reference: res.Reference}
			return nil

		case errors.Is(err, gateway.ErrDeclined):
			out = chargeOutcome{status: StatusFailed, reference: res.Reference, lastError: err.Error()}
			return nil // final: a refusal is an answer

		case errors.Is(err, gateway.ErrIndeterminate):
			log.Printf("payment-worker: no answer for payment %s; marking UNKNOWN without retrying: %v", pay.ID, err)
			out = chargeOutcome{status: StatusUnknown, lastError: err.Error()}
			return nil // do NOT retry: the charge may be in flight

		default:
			log.Printf("payment-worker: charge for payment %s failed, will retry: %v", pay.ID, err)
			return err // known not to have charged — retrying is safe
		}
	})

	if retryErr != nil {
		// Every attempt failed in a retryable way. We never saw the gateway
		// commit anything, but we also cannot prove it did not, so UNKNOWN is
		// the honest state.
		log.Printf("payment-worker: payment %s exhausted %d attempts; marking UNKNOWN: %v",
			pay.ID, p.policy.MaxAttempts, retryErr)
		return chargeOutcome{
			status:    StatusUnknown,
			lastError: fmt.Sprintf("exhausted %d attempts: %v", p.policy.MaxAttempts, retryErr),
		}
	}

	return out
}
