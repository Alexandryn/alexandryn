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
// never using package-level time.Now.
type Clock interface {
	Now() time.Time
}

// Queue is the enqueue-and-query API that job handlers use. It is
// constructed once and injected rather than using package-level functions.
type Queue struct {
	store    *Store
	registry *Registry
	ids      domain.IDGenerator
	clock    Clock
	live     *liveJobs
}

// SystemClock is the real wall clock — production's Clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// NewQueue builds a Queue over a store and registry.
func NewQueue(store *Store, registry *Registry, ids domain.IDGenerator, clock Clock, live *liveJobs) *Queue {
	return &Queue{store: store, registry: registry, ids: ids, clock: clock, live: live}
}

// Register binds a handler to a kind. Call at process startup.
func (q *Queue) Register(kind Kind, maxAttempts int, handler HandlerFunc) {
	q.registry.Register(kind, maxAttempts, handler)
}

// Enqueue schedules work for a registered kind. It returns InvalidInput
// if kind has no registered handler — an unregistered kind fails immediately at
// enqueue time, rather than sitting unclaimed. payload
// must be JSON-serializable and must not itself carry secrets.
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
// progress, timestamps) or a NotFound error.
func (q *Queue) GetJob(ctx context.Context, id ID) (Job, error) {
	return q.store.GetJob(ctx, id)
}

// ListJobs returns jobs matching filter, oldest first.
func (q *Queue) ListJobs(ctx context.Context, filter JobFilter) ([]Job, error) {
	return q.store.ListJobs(ctx, filter)
}

// CountByState returns current job counts grouped by status.
func (q *Queue) CountByState(ctx context.Context) (map[State]int, error) {
	return q.store.CountByState(ctx)
}

// CancelJob cancels a queued or running job, moving it to dead_letter,
// and immediately aborts the handler if a worker in this process is
// running it.
func (q *Queue) CancelJob(ctx context.Context, id ID) error {
	if err := q.store.CancelJob(ctx, id, q.clock.Now()); err != nil {
		return err
	}
	q.live.cancel(id)
	return nil
}

// RetryJob re-enqueues a job by inserting a new record with attempts = 0.
func (q *Queue) RetryJob(ctx context.Context, id ID) (ID, error) {
	newID := ID(q.ids.NewID())
	return q.store.RetryJob(ctx, id, newID, q.clock.Now())
}

// ClearCompleted deletes completed jobs older than cutoff.
func (q *Queue) ClearCompleted(ctx context.Context, olderThan time.Time) (int64, error) {
	return q.store.ClearCompleted(ctx, olderThan)
}
