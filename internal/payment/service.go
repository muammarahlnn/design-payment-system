package payment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/muammarahlnn/design-payment-system/internal/event"
	"github.com/muammarahlnn/design-payment-system/internal/idempotency"
	"github.com/muammarahlnn/design-payment-system/internal/infra/postgres"
	"github.com/muammarahlnn/design-payment-system/internal/outbox"
)

// ErrIdempotencyKeyReused means the key already exists, but the request body
// differs from the one that created the payment. The client has a bug: it
// reused a key for a different charge. Returning the first payment would hide
// that, and creating a second one would break the idempotency promise. So we
// reject it and let the client see the mistake.
var ErrIdempotencyKeyReused = errors.New("idempotency key reused with a different request")

// errKeyTaken is internal. It travels out of the transaction closure purely to
// force a rollback when another request won the key, so the payment row this
// transaction inserted disappears again. It never reaches the caller.
var errKeyTaken = errors.New("idempotency key already taken")

// CreateInput carries a payment request into the service. Shape validation
// (required fields, positive amount, ...) is the handler's job; this is where
// payment business rules live.
type CreateInput struct {
	IdempotencyKey string
	CustomerID     string
	Amount         int64 // whole rupiah, not sen (IDR is zero-decimal)
	Currency       string
	PaymentMethod  string
}

// Service holds the payment business logic.
type Service struct {
	db       *sql.DB
	payments *Repository
	keys     *idempotency.Repository
	outbox   *outbox.Repository
}

func NewService(db *sql.DB, payments *Repository, keys *idempotency.Repository, outboxRepo *outbox.Repository) *Service {
	return &Service{db: db, payments: payments, keys: keys, outbox: outboxRepo}
}

// Create records a new payment with status PENDING, or returns the payment an
// earlier request with the same idempotency key already created.
//
// Three rows are written in one transaction, so all three commit together:
//
//	payments          the payment itself
//	idempotency_keys  the client's key, so a retry cannot charge twice
//	outbox            the payment.created event, for the relay to ship
//
// Either all of them land or none do. A payment cannot exist without its key
// (a retry would double-charge), and it cannot exist without its event (the
// worker would never hear about it).
//
// The bool reports whether this request made the payment (true) or replayed an
// existing one (false), so the handler can answer 201 or 200.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Payment, bool, error) {
	hash := fingerprint(in)

	p := &Payment{
		CustomerID:    in.CustomerID,
		Amount:        in.Amount,
		Currency:      in.Currency,
		PaymentMethod: in.PaymentMethod,
	}

	err := postgres.RunInTx(ctx, s.db, func(tx *sql.Tx) error {
		// The payment must exist before the key can point at it, because
		// idempotency_keys.payment_id is a foreign key.
		if err := s.payments.WithTx(tx).Insert(ctx, p); err != nil {
			return err
		}

		claimed, err := s.keys.WithTx(tx).Claim(ctx, &idempotency.Key{
			Key:         in.IdempotencyKey,
			RequestHash: hash,
			PaymentID:   p.ID,
		})
		if err != nil {
			return err
		}
		if !claimed {
			// Another request holds this key. Roll back, which also undoes
			// the payment we just inserted, then replay the winner below.
			return errKeyTaken
		}

		payload, err := json.Marshal(event.PaymentCreated{
			PaymentID:     p.ID,
			CustomerID:    p.CustomerID,
			Amount:        p.Amount,
			Currency:      p.Currency,
			PaymentMethod: p.PaymentMethod,
			Status:        p.Status,
			CreatedAt:     p.CreatedAt,
		})
		if err != nil {
			return fmt.Errorf("marshal %s event: %w", event.TopicPaymentCreated, err)
		}

		return s.outbox.WithTx(tx).Insert(ctx, &outbox.Message{
			AggregateType: event.AggregateTypePayment,
			AggregateID:   p.ID,
			EventType:     event.TopicPaymentCreated,
			Payload:       payload,
		})
	})

	if err == nil {
		return p, true, nil
	}
	if !errors.Is(err, errKeyTaken) {
		return nil, false, fmt.Errorf("create payment: %w", err)
	}

	return s.replay(ctx, in.IdempotencyKey, hash)
}

// replay returns the payment an earlier request created under this key.
func (s *Service) replay(ctx context.Context, key, hash string) (*Payment, bool, error) {
	existing, err := s.keys.Get(ctx, key)
	if err != nil {
		return nil, false, fmt.Errorf("replay payment: %w", err)
	}

	if existing.RequestHash != hash {
		return nil, false, ErrIdempotencyKeyReused
	}

	p, err := s.payments.GetByID(ctx, existing.PaymentID)
	if err != nil {
		return nil, false, fmt.Errorf("replay payment: %w", err)
	}
	return p, false, nil
}

// fingerprint hashes the request fields that define the charge. It hashes the
// parsed values rather than the raw JSON body, so whitespace or a different
// field order still counts as the same request.
func fingerprint(in CreateInput) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%d|%s|%s", in.CustomerID, in.Amount, in.Currency, in.PaymentMethod)
	return hex.EncodeToString(h.Sum(nil))
}
