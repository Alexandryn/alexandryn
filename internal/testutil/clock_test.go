package testutil_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// FR-5 canary (backend-test-harness.md): a FakeClock constructed at a known
// time, read twice with Advance() called between the reads, observes
// exactly the advanced duration — proven without any real time.Sleep.
func TestFakeClock_AdvanceIsObservedExactly(t *testing.T) {
	start := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	clock := testutil.NewFakeClock(start)

	first := clock.Now()
	if !first.Equal(start) {
		t.Fatalf("Now() before any Advance = %v, want %v", first, start)
	}

	clock.Advance(90 * time.Second)

	second := clock.Now()
	elapsed := second.Sub(first)
	if elapsed != 90*time.Second {
		t.Fatalf("elapsed between reads = %v, want exactly 90s", elapsed)
	}
}

func TestFakeClock_MultipleAdvancesAccumulate(t *testing.T) {
	clock := testutil.NewFakeClock(time.Unix(0, 0))

	clock.Advance(time.Minute)
	clock.Advance(30 * time.Second)

	want := time.Unix(0, 0).Add(90 * time.Second)
	if got := clock.Now(); !got.Equal(want) {
		t.Fatalf("Now() after two advances = %v, want %v", got, want)
	}
}
