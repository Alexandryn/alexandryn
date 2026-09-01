package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Clock is the one time method this package needs. testutil.FakeClock
// satisfies it; production passes the real wall clock. It is injected,
// never a package-level time.Now (backend-test-harness.md FR-5).
type Clock interface {
	Now() time.Time
}

// Queue is the enqueue-and-query API future job handlers use. It is
// constructed once and injected (backend-service-lifecycle.md FR-2) —
// not a package with global Register/Enqueue functions, which would need
// the package-level state that rule forbids.
type Queue struct {
	store    *Store
	registry *Registry
	ids      domain.IDGenerator
	clock    Clock
}

// SystemClock is the real wall clock — production's Clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// NewQueue builds a Queue over a store and registry.
func NewQueue(store *Store, registry *Registry, ids domain.IDGenerator, clock Clock) *Queue {
	return &Queue{store: store, registry: registry, ids: ids, clock: clock}
}

// Register binds a handler to a kind (FR-2). Call at process startup.
func (q *Queue) Register(kind Kind, maxAttempts int, handler HandlerFunc) {
	q.registry.Register(kind, maxAttempts, handler)
}

// Enqueue schedules work for a registered kind. It returns InvalidInput
// if kind has no registered handler — a typo'd kind fails here, at
// enqueue time, rather than sitting unclaimed forever (FR-2). payload
// must be JSON-serializable and must not itself carry a secret (FR-3) —
// a discipline for the caller, not something this method can enforce.
func (q *Queue) Enqueue(ctx context.Context, kind Kind, payload any) (ID, error) {
	reg, ok := q.registry.lookup(kind)
	if !ok {
		return "", &domain.Error{
			Category: domain.InvalidInput,
			Message:  fmt.Sprintf("no handler is registered for job kind %q", kind),
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", &domain.Error{
			Category: domain.InvalidInput,
			Message:  "job payload is not JSON-serializable",
			Err:      err,
		}
	}

	id := ID(q.ids.NewID())
	now := q.clock.Now()
	if err := q.store.Enqueue(ctx, NewJob{
		ID:          id,
		Kind:        kind,
		Payload:     raw,
		MaxAttempts: reg.maxAttempts,
		AvailableAt: now,
		Now:         now,
	}); err != nil {
		return "", err
	}
	return id, nil
}

// GetJob returns one job's full row (status, attempts, last_error,
// progress, timestamps) or a NotFound error (FR-9).
func (q *Queue) GetJob(ctx context.Context, id ID) (Job, error) {
	return q.store.GetJob(ctx, id)
}

// ListJobs returns jobs matching filter, oldest first (FR-9).
func (q *Queue) ListJobs(ctx context.Context, filter JobFilter) ([]Job, error) {
	return q.store.ListJobs(ctx, filter)
}
