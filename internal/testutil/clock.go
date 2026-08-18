// Package testutil provides fakes — a controllable clock, an in-memory
// filesystem, a deterministic ID sequence, and a log spy — that let a test
// observe behavior without touching real time, real disk, or real
// randomness. Test-only: no production code imports this package.
package testutil

import (
	"sync"
	"time"
)

// FakeClock is a settable, advanceable clock for tests that would
// otherwise depend on real elapsed time.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock constructs a FakeClock fixed at start.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

// Now returns the clock's current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the fake clock forward by d.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
