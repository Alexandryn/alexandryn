package jobs

import "time"

// Backoff computes the wait before a failed job's next attempt.
type Backoff struct {
	Base time.Duration
	Max  time.Duration
}

// For returns the delay before the retry that follows the given attempt
// count. attempts is the value already incremented at claim time,
// so the first retry passes attempts == 1 and waits close to Base, not
// 2*Base — the exponent is attempts-1.
//
// The uncapped exponential is min(Base * 2^(attempts-1), Max); the
// result is that value scaled by a jitter factor in [0.8, 1.2), i.e.
// ±20%, so a batch of jobs failing together does not retry in
// lockstep. jitter is a value in [0, 1) the caller supplies
// (rand.Float64() in the poll loop) rather than this function reaching
// for global randomness, so unit tests remain deterministic.
func (b Backoff) For(attempts int, jitter float64) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if jitter < 0 {
		jitter = 0
	}
	if jitter >= 1 {
		jitter = 0.999999
	}

	delay := b.Base
	for i := 1; i < attempts; i++ {
		delay *= 2
		if b.Max > 0 && delay >= b.Max {
			delay = b.Max
			break
		}
	}
	if b.Max > 0 && delay > b.Max {
		delay = b.Max
	}

	factor := 0.8 + 0.4*jitter
	return time.Duration(float64(delay) * factor)
}
