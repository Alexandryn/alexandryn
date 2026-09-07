package http

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/observability"
)

type observabilityRefs struct {
	eventStore atomic.Pointer[observability.EventStore]
	jobQueue   atomic.Pointer[jobs.Queue]
	jobSystem  atomic.Pointer[jobs.System]
	metricsReg atomic.Pointer[observability.Registry]
}

// SetEventStore stores the EventStore reference.
func (r *PoolRef) SetEventStore(s *observability.EventStore) {
	r.observability.eventStore.Store(s)
}

// GetEventStore returns the EventStore reference if set.
func (r *PoolRef) GetEventStore() (*observability.EventStore, bool) {
	v := r.observability.eventStore.Load()
	return v, v != nil
}

// SetJobQueue stores the jobs.Queue reference.
func (r *PoolRef) SetJobQueue(q *jobs.Queue) {
	r.observability.jobQueue.Store(q)
}

// GetJobQueue returns the jobs.Queue reference if set.
func (r *PoolRef) GetJobQueue() (*jobs.Queue, bool) {
	v := r.observability.jobQueue.Load()
	return v, v != nil
}

// SetJobSystem stores the jobs.System reference.
func (r *PoolRef) SetJobSystem(s *jobs.System) {
	r.observability.jobSystem.Store(s)
}

// GetJobSystem returns the jobs.System reference if set.
func (r *PoolRef) GetJobSystem() (*jobs.System, bool) {
	v := r.observability.jobSystem.Load()
	return v, v != nil
}

// SetMetricsRegistry stores the observability.Registry reference.
func (r *PoolRef) SetMetricsRegistry(reg *observability.Registry) {
	r.observability.metricsReg.Store(reg)
}

// GetMetricsRegistry returns the observability.Registry reference if set.
func (r *PoolRef) GetMetricsRegistry() (*observability.Registry, bool) {
	v := r.observability.metricsReg.Load()
	return v, v != nil
}

// LazyActivityEventsHandler wraps ActivityEventsHandler with lazy dependency resolution.
func LazyActivityEventsHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store, ok := ref.GetEventStore()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		ActivityEventsHandler(store).ServeHTTP(w, r)
	})
}

// LazyActivityPauseAllHandler wraps ActivityPauseAllHandler with lazy dependency resolution.
func LazyActivityPauseAllHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sys, _ := ref.GetJobSystem()
		ActivityPauseAllHandler(sys).ServeHTTP(w, r)
	})
}

// LazyActivityJobCancelHandler wraps ActivityJobCancelHandler with lazy dependency resolution.
func LazyActivityJobCancelHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queue, ok := ref.GetJobQueue()
		if !ok {
			WriteError(w, domain.Unavailable, "job system not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		ActivityJobCancelHandler(queue).ServeHTTP(w, r)
	})
}

// LazyActivityJobRetryHandler wraps ActivityJobRetryHandler with lazy dependency resolution.
func LazyActivityJobRetryHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queue, ok := ref.GetJobQueue()
		if !ok {
			WriteError(w, domain.Unavailable, "job system not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		ActivityJobRetryHandler(queue).ServeHTTP(w, r)
	})
}

// LazyActivityClearCompletedHandler wraps ActivityClearCompletedHandler with lazy dependency resolution.
func LazyActivityClearCompletedHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queue, ok := ref.GetJobQueue()
		if !ok {
			WriteError(w, domain.Unavailable, "job system not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		ActivityClearCompletedHandler(queue, time.Now).ServeHTTP(w, r)
	})
}

// LazyDiagnosticsHandler wraps DiagnosticsHandler with lazy dependency resolution.
func LazyDiagnosticsHandler(ref *PoolRef, startTime time.Time, commit, buildTime string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reg, _ := ref.GetMetricsRegistry()
		DiagnosticsHandler(reg, startTime, commit, buildTime).ServeHTTP(w, r)
	})
}
