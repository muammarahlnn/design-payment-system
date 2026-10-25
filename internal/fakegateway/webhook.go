package fakegateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/muammarahlnn/design-payment-system/internal/webhook"
)

// Notifier delivers a settlement callback back to payment-service.
type Notifier struct {
	url    string
	secret string
	http   *http.Client
}

func NewNotifier(url, secret string) *Notifier {
	return &Notifier{
		url:    url,
		secret: secret,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

type callback struct {
	EventID   string `json:"event_id"`
	PaymentID string `json:"payment_id"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

// Notify delivers one signed callback and reports what payment-service said.
//
// It is synchronous on purpose: this fires when a human calls the settle
// endpoint, and that caller should see immediately whether the callback landed
// rather than having to go read logs.
//
// paymentID is the idempotency key the merchant sent. Real gateways echo the
// merchant's own reference back for exactly this reason: it is the only id the
// merchant can always resolve, including for a charge whose response they
// never received.
func (n *Notifier) Notify(ctx context.Context, paymentID string, c Charge, status string) (eventID string, httpStatus int, err error) {
	eventID = "evt_" + uuid.NewString()

	body, err := json.Marshal(callback{
		EventID:   eventID,
		PaymentID: paymentID,
		Reference: c.Reference,
		Status:    status,
	})
	if err != nil {
		return eventID, 0, fmt.Errorf("marshal callback: %w", err)
	}

	ts, sig := webhook.Sign(n.secret, body, time.Now())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return eventID, 0, fmt.Errorf("build callback: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(webhook.HeaderTimestamp, ts)
	req.Header.Set(webhook.HeaderSignature, sig)

	resp, err := n.http.Do(req)
	if err != nil {
		// A real gateway would queue this and retry with backoff. Here it is
		// enough to report it: step 7's reconciliation is the real answer to
		// a callback that never lands.
		return eventID, 0, fmt.Errorf("deliver callback: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("fake-gateway: delivered %s callback for payment %s -> HTTP %d",
		status, paymentID, resp.StatusCode)
	return eventID, resp.StatusCode, nil
}
