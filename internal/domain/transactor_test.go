package domain_test

import (
	"context"
	"errors"
	"sync"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// fakeTransactor is an in-memory domain.Transactor — it calls fn directly
// (no real rollback semantics; that's internal/persistence/postgres's own
// implementation, T24's job) but records how many times InTx was invoked,
// which is what this package's tests need to prove a cross-aggregate
// operation was actually composed through it rather than left as two
// unsequenced calls.
type fakeTransactor struct {
	mu    sync.Mutex
	calls int
}

func (t *fakeTransactor) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	t.mu.Lock()
	t.calls++
	t.mu.Unlock()
	return fn(ctx)
}

func (t *fakeTransactor) callCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

var _ domain.Transactor = (*fakeTransactor)(nil)

// failingTransactor's InTx never calls fn — it simulates the transaction
// itself failing to begin (e.g. pool exhaustion), distinct from fn
// returning its own error.
type failingTransactor struct{}

func (failingTransactor) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return errors.New("simulated: failed to begin transaction")
}

var _ domain.Transactor = (failingTransactor{})
