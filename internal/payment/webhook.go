package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/muammarahlnn/design-payment-system/internal/inbox"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
)

// WebhookConsumerName identifies webhook deliveries in processed_events. It is
// separate from the worker's name so the two dedup independently — the same
// table serves both, which is why its key is (consumer, event_id).
const WebhookConsumerName = "payment-webhook"

// ErrUnknownStatus means the gateway sent an outcome we do not model.
var ErrUnknownStatus = errors.New("unrecognized webhook status")

// WebhookEvent is one delivery from the gateway.
type WebhookEvent struct {
	// EventID is the gateway's id for this delivery. Gateways retry webhooks,
	// so this is what makes handling them idempotent.
	EventID string
	// PaymentID is OUR id, echoed back. It is the lookup key rather than the
	// gateway's reference, because a payment that timed out is UNKNOWN with a
	// NULL gateway_reference — and resolving those is what webhooks are for.
	PaymentID string
	Reference string
	Status    string
}

// WebhookService applies gateway callbacks to payments.
type WebhookService struct {
	db       *sql.DB
	payments *Repository
	inbox    *inbox.Repository
}

func NewWebhookService(db *sql.DB, payments *Repository, inboxRepo *inbox.Repository) *WebhookService {
	return &WebhookService{db: db, payments: payments, inbox: inboxRepo}
}

// Handle settles a payment from a gateway callback.
//
// Everything happens in one transaction: the dedup marker and the status
// change commit together, so a delivery can never be recorded as handled
// without its effect, or the reverse.
func (s *WebhookService) Handle(ctx context.Context, e WebhookEvent) error {
	var next string
	switch e.Status {
	case StatusSucceeded, StatusFailed:
		next = e.Status
	default:
		return fmt.Errorf("%w: %q", ErrUnknownStatus, e.Status)
	}

	return postgres.RunInTx(ctx, s.db, func(tx *sql.Tx) error {
		newly, err := s.inbox.WithTx(tx).MarkProcessed(ctx, WebhookConsumerName, e.EventID)
		if err != nil {
			return err
		}
		if !newly {
			log.Printf("webhook: delivery %s already handled for payment %s; skipping", e.EventID, e.PaymentID)
			return nil
		}

		settled, err := s.payments.WithTx(tx).SettleStatus(ctx, e.PaymentID, next, e.Reference)
		if err != nil {
			return err
		}
		if !settled {
			// Either the payment is already terminal, or it does not exist.
			// Neither is an error worth failing the delivery over: the gateway
			// would just retry something that can never change.
			log.Printf("webhook: payment %s not settled to %s (already final, or unknown id)", e.PaymentID, next)
			return nil
		}

		log.Printf("webhook: payment %s -> %s (delivery %s)", e.PaymentID, next, e.EventID)
		return nil
	})
}
