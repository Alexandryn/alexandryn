package jobs

import (
	"testing"
	"time"
)

func TestBackoff_FirstRetryIsAboutBase(t *testing.T) {
	b := Backoff{Base: 5 * time.Second, Max: 5 * time.Minute}

	// attempts == 1 is the first retry: exponent attempts-1 == 0, so the
	// uncapped delay is exactly Base.
	if got := b.For(1, 0.5); got != 5*time.Second {
		t.Fatalf("For(1, 0.5) = %v, want 5s (Base, no jitter offset at 0.5)", got)
	}
}

func TestBackoff_DoublesPerAttempt(t *testing.T) {
	b := Backoff{Base: 5 * time.Second, Max: time.Hour}
	// jitter 0.5 -> factor 1.0, so the raw exponential shows through.
	cases := map[int]time.Duration{
		1: 5 * time.Second,
		2: 10 * time.Second,
		3: 20 * time.Second,
		4: 40 * time.Second,
		5: 80 * time.Second,
	}
	for attempts, want := range cases {
		if got := b.For(attempts, 0.5); got != want {
			t.Errorf("For(%d, 0.5) = %v, want %v", attempts, got, want)
		}
	}
}

func TestBackoff_CapsAtMax(t *testing.T) {
	b := Backoff{Base: 5 * time.Second, Max: 5 * time.Minute}
	// 5s * 2^9 = 2560s, well over the 300s cap.
	if got := b.For(10, 0.5); got != 5*time.Minute {
		t.Fatalf("For(10, 0.5) = %v, want the 5m cap", got)
	}
}

func TestBackoff_JitterStaysWithinTwentyPercent(t *testing.T) {
	b := Backoff{Base: 10 * time.Second, Max: time.Hour}
	base := 10 * time.Second
	lo := time.Duration(float64(base) * 0.8)
	hi := time.Duration(float64(base) * 1.2)

	for _, j := range []float64{0, 0.01, 0.25, 0.5, 0.75, 0.99, -3, 4} {
		got := b.For(1, j)
		if got < lo || got > hi {
			t.Errorf("For(1, %v) = %v, outside [%v, %v]", j, got, lo, hi)
		}
	}
}

func TestBackoff_JitterEndpoints(t *testing.T) {
	b := Backoff{Base: 100 * time.Second, Max: time.Hour}
	if got := b.For(1, 0); got != 80*time.Second {
		t.Errorf("jitter 0 -> %v, want 80s (-20%%)", got)
	}
	got := b.For(1, 0.999999)
	// approaches +20% but never reaches it
	if got <= 119*time.Second || got > 120*time.Second {
		t.Errorf("jitter ~1 -> %v, want just under 120s", got)
	}
}
