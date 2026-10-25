// Package retry runs an operation with bounded, backing-off, jittered retries.
package retry

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// Policy configures Do. The zero Policy runs the operation exactly once.
type Policy struct {
	MaxAttempts int           // total attempts including the first (min 1)
	BaseDelay   time.Duration // delay after the first failure
	MaxDelay    time.Duration // cap on any single delay (0 = uncapped)
	Multiplier  float64       // per-attempt growth factor (min 1)
}

// Do calls op until it returns nil or attempts run out, sleeping between
// attempts with capped exponential backoff plus equal jitter. It returns
// ctx.Err() if ctx is cancelled during a wait, otherwise the last error.
func Do(ctx context.Context, p Policy, op func() error) error {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}
	if p.Multiplier < 1 {
		p.Multiplier = 1
	}

	var err error
	for attempt := 1; ; attempt++ {
		if err = op(); err == nil {
			return nil
		}
		if attempt >= p.MaxAttempts {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.backoff(attempt)):
		}
	}
}

func (p Policy) backoff(attempt int) time.Duration {
	d := float64(p.BaseDelay) * math.Pow(p.Multiplier, float64(attempt-1))
	if p.MaxDelay > 0 && d > float64(p.MaxDelay) {
		d = float64(p.MaxDelay)
	}
	if d <= 0 {
		return 0
	}
	half := time.Duration(d / 2)
	return half + time.Duration(rand.Int64N(int64(half)+1))
}
