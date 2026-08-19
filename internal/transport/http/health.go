package http

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Pinger is the minimal interface /readyz needs from whatever holds the
// connection pool. A real *pgxpool.Pool already implements this natively;
// tests use a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// PoolRef is an atomically-held reference to the pool
// (backend-service-lifecycle.md FR-1's readiness mechanism,
// backend-persistence.md FR-1). It starts empty at process start and is
// populated once Postgres is connected and migrated.
type PoolRef struct {
	p atomic.Pointer[Pinger]
}

// Set stores p as the current reference.
func (r *PoolRef) Set(p Pinger) {
	r.p.Store(&p)
}

// Get returns the current reference and whether one has been Set yet.
func (r *PoolRef) Get() (Pinger, bool) {
	stored := r.p.Load()
	if stored == nil {
		return nil, false
	}
	return *stored, true
}

// Healthz answers "is the process alive" — 200 the instant the process
// can accept HTTP connections, independent of PostgreSQL state. It takes
// the same pool reference /readyz reads, for constructor symmetry with
// how cmd/server wires both, but never reads it — a health check that
// blocked on the database would defeat the alive/ready distinction
// (backend-http-transport.md FR-5, architecture-system.md FR-7).
func Healthz(_ *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
}

// Readyz answers "can this process actually serve data": 503 naming
// "not yet started" while ref is unset, 200 once set and a liveness
// check succeeds, 503 naming "lost the connection" if it fails — three
// states, distinguished by body text, never PostgreSQL's own connection
// error, which can embed DATABASE_URL (backend-http-transport.md FR-5).
func Readyz(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		pinger, ok := ref.Get()
		if !ok {
			WriteError(w, domain.Unavailable, "not yet started", id)
			return
		}

		if err := pinger.Ping(r.Context()); err != nil {
			WriteError(w, domain.Unavailable, "lost the connection", id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
}
