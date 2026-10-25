// Package webhook holds the signing scheme shared by the sender (fake-gateway)
// and the receiver (payment-service), so the two cannot drift apart.
//
// The scheme follows Stripe's: an HMAC-SHA256 over "<timestamp>.<raw body>",
// sent alongside the timestamp. That buys two things a plain shared token
// (Xendit's style) does not:
//
//	authenticity — only a holder of the secret can produce the digest
//	freshness    — the timestamp is inside the signed material, so an attacker
//	               who captures a valid request cannot replay it later without
//	               the signature failing
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Headers the sender sets and the receiver reads.
const (
	HeaderSignature = "X-Signature"
	HeaderTimestamp = "X-Timestamp"
)

// DefaultTolerance is how old a signed request may be. Long enough to absorb
// clock skew and a slow network, short enough that a captured request stops
// being useful quickly.
const DefaultTolerance = 5 * time.Minute

var (
	ErrMissingSignature = errors.New("missing signature or timestamp header")
	ErrBadTimestamp     = errors.New("timestamp is not a unix seconds value")
	ErrStale            = errors.New("signature timestamp is outside the tolerance window")
	ErrBadSignature     = errors.New("signature does not match")
)

// Sign returns the timestamp and hex digest to send with body.
func Sign(secret string, body []byte, at time.Time) (timestamp, signature string) {
	ts := strconv.FormatInt(at.Unix(), 10)
	return ts, digest(secret, ts, body)
}

// Verify checks a received signature against the raw body.
//
// Pass the body exactly as it arrived on the wire. Re-marshalling a decoded
// struct will not reproduce the same bytes — field order, whitespace, and
// number formatting all shift — and the signature will fail for a request that
// was perfectly valid.
func Verify(secret, timestamp, signature string, body []byte, now time.Time, tolerance time.Duration) error {
	if timestamp == "" || signature == "" {
		return ErrMissingSignature
	}

	secs, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadTimestamp, err)
	}

	age := now.Sub(time.Unix(secs, 0))
	if age < 0 {
		age = -age // tolerate a sender whose clock runs slightly ahead
	}
	if age > tolerance {
		return fmt.Errorf("%w: %s old", ErrStale, age.Truncate(time.Second))
	}

	expected := digest(secret, timestamp, body)

	// hmac.Equal, never ==. A byte-by-byte comparison returns early on the
	// first mismatch, and that timing difference leaks how much of a guessed
	// signature was correct, which is enough to forge one a byte at a time.
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return ErrBadSignature
	}
	return nil
}

func digest(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
