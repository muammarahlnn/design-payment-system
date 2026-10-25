package event

import "time"

// TopicPaymentCreated is published by payment-service whenever a payment is
// persisted, and consumed by payment-worker. TopicPaymentCreatedDLQ is where
// the worker parks events it cannot process after retries — nothing moves
// messages there automatically, the consumer produces to it.
const (
	TopicPaymentCreated    = "payment.created"
	TopicPaymentCreatedDLQ = "payment.created.dlq"
)

// AggregateTypePayment labels outbox rows that describe a payment.
const AggregateTypePayment = "payment"

// PaymentCreated is the JSON wire format for the payment.created event.
// Amount is whole rupiah, matching the payments table.
type PaymentCreated struct {
	PaymentID     string    `json:"payment_id"`
	CustomerID    string    `json:"customer_id"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	PaymentMethod string    `json:"payment_method"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
