package testutil

import "sync"

// FakeIDGenerator produces a fixed, deterministic sequence of IDs — the
// same sequence on every test run, never real randomness.
type FakeIDGenerator struct {
	mu  sync.Mutex
	ids []string
	i   int
}

// NewFakeIDGenerator constructs a FakeIDGenerator that yields ids in order,
// one per NewID call.
func NewFakeIDGenerator(ids ...string) *FakeIDGenerator {
	return &FakeIDGenerator{ids: ids}
}

// NewID returns the next ID in the fixed sequence. It panics once the
// sequence is exhausted — a test that needs more IDs than it declared
// should say so by failing loudly, not by silently repeating or returning
// an empty string.
func (g *FakeIDGenerator) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.i >= len(g.ids) {
		panic("testutil: FakeIDGenerator sequence exhausted")
	}
	id := g.ids[g.i]
	g.i++
	return id
}
