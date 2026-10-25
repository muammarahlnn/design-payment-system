package webhook

import (
	"errors"
	"testing"
	"time"
)

const secret = "test-secret"

func TestVerifyAcceptsAFreshSignature(t *testing.T) {
	body := []byte(`{"payment_id":"abc","status":"SUCCEEDED"}`)
	now := time.Now()

	ts, sig := Sign(secret, body, now)
	if err := Verify(secret, ts, sig, body, now, DefaultTolerance); err != nil {
		t.Fatalf("fresh signature rejected: %v", err)
	}
}

func TestVerifyRejectsATamperedBody(t *testing.T) {
	body := []byte(`{"payment_id":"abc","status":"SUCCEEDED"}`)
	now := time.Now()
	ts, sig := Sign(secret, body, now)

	tampered := []byte(`{"payment_id":"abc","status":"FAILED"}`)
	if err := Verify(secret, ts, sig, tampered, now, DefaultTolerance); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("want ErrBadSignature, got %v", err)
	}
}

func TestVerifyRejectsTheWrongSecret(t *testing.T) {
	body := []byte(`{}`)
	now := time.Now()
	ts, sig := Sign(secret, body, now)

	if err := Verify("other-secret", ts, sig, body, now, DefaultTolerance); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("want ErrBadSignature, got %v", err)
	}
}

// A captured request replayed an hour later must fail, even though its
// signature is genuine — that is the point of signing the timestamp.
func TestVerifyRejectsAReplay(t *testing.T) {
	body := []byte(`{}`)
	signedAt := time.Now()
	ts, sig := Sign(secret, body, signedAt)

	later := signedAt.Add(time.Hour)
	if err := Verify(secret, ts, sig, body, later, DefaultTolerance); !errors.Is(err, ErrStale) {
		t.Fatalf("want ErrStale, got %v", err)
	}
}

func TestVerifyRejectsMissingHeaders(t *testing.T) {
	if err := Verify(secret, "", "", []byte(`{}`), time.Now(), DefaultTolerance); !errors.Is(err, ErrMissingSignature) {
		t.Fatalf("want ErrMissingSignature, got %v", err)
	}
}
