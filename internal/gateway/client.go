// Package gateway is the client side of the payment provider. The worker uses
// it to submit a charge.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrDeclined means the gateway answered, and the answer was no. It is a
// final business outcome, not a transport failure, so the caller must not
// retry it.
var ErrDeclined = errors.New("gateway declined the charge")

// ErrIndeterminate means we never got an answer, so we do not know whether the
// charge happened. The caller must not retry it inline and must not treat the
// payment as un-charged.
//
// Retrying would be safe from a double-charge standpoint — the idempotency key
// sees to that. It is a bad idea for three other reasons: the first request may
// still be executing, and a concurrent one with the same key can hit a race in
// the provider's own idempotency handling; a provider that is timing out is
// overloaded, and more requests make that worse; and a retry against a timing-
// out provider will almost certainly time out too, so you burn another full
// timeout to learn nothing. Reconciliation asks later, once it has recovered.
var ErrIndeterminate = errors.New("gateway gave no answer; the charge may or may not have happened")

// Charge statuses the gateway can return.
const (
	StatusAccepted = "ACCEPTED"
	StatusDeclined = "DECLINED"
)

// ChargeRequest is what the worker submits.
type ChargeRequest struct {
	// IdempotencyKey is the payment id. It must be stable across retries:
	// that is the whole reason a retried charge does not take the money twice.
	IdempotencyKey string
	Amount         int64
	Currency       string
	PaymentMethod  string
}

// ChargeResult is the gateway's answer.
type ChargeResult struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

// ErrNoSuchCharge means the gateway has no record of this charge at all.
//
// For a real provider that is a strong statement: the charge never happened.
// Reconciliation still does not act on it automatically — see the reconciler
// for why.
var ErrNoSuchCharge = errors.New("gateway has no record of this charge")

// Client talks to the payment gateway over HTTP.
type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
	}
}

// Charge submits a charge and returns the gateway's answer.
//
// Three outcomes the caller must tell apart:
//
//	nil error         the gateway accepted it
//	ErrDeclined       the gateway refused — final, do not retry
//	ErrIndeterminate  no answer at all — do not retry inline; the payment
//	                  goes to UNKNOWN and reconciliation settles it
//	any other error   the request failed in a way we know did not charge
//	                  (connection refused, 5xx) — safe to retry inline
func (c *Client) Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	body, err := json.Marshal(map[string]any{
		"amount":         req.Amount,
		"currency":       req.Currency,
		"payment_method": req.PaymentMethod,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal charge request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/charges", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build charge request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, classify(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusPaymentRequired {
		return nil, fmt.Errorf("gateway returned %d", resp.StatusCode)
	}

	var result ChargeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode charge response: %w", err)
	}

	// A decline still carries a reference, so the caller can record which
	// gateway charge said no. The error marks it final, not retryable.
	if resp.StatusCode == http.StatusPaymentRequired {
		return &result, ErrDeclined
	}
	return &result, nil
}

// GetCharge asks the gateway what it believes about a charge. This is the
// query side that reconciliation is built on: our database can be wrong or
// stale, and the provider is the authority on whether money moved.
func (c *Client) GetCharge(ctx context.Context, paymentID string) (*ChargeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/charges/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("build get-charge request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classify(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoSuchCharge
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gateway returned %d", resp.StatusCode)
	}

	var result ChargeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode charge: %w", err)
	}
	return &result, nil
}

// classify decides whether a transport failure left the charge in doubt.
//
// A timeout is the dangerous case: the request may have reached the gateway and
// may be mid-charge. Anything else here — connection refused, DNS failure, TLS
// error — failed before the gateway could act on it, so the charge definitely
// did not happen and retrying is safe.
//
// Dial timeouts technically also never reach the gateway, but telling them
// apart from response timeouts is fragile. We take the conservative reading and
// call every timeout indeterminate: the cost is that reconciliation sometimes
// checks a charge that never existed, which is cheap. The opposite mistake is
// not.
func classify(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrIndeterminate, err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%w: %v", ErrIndeterminate, err)
	}

	return fmt.Errorf("call gateway: %w", err)
}
