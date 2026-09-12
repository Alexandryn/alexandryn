package jobs

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// System is the job subsystem as one wired unit: the enqueue/query
// Queue that job handlers use, and the worker pool that runs
// them. cmd/server constructs one against the shared pool at startup
// and drives its lifecycle during application runtime.
type System struct {
	queue  *Queue
	engine *Engine
}

// NewSystem builds the subsystem over the process's shared connection
// pool. ids is the production UUID generator; clock is the real wall
// clock; cfg's zero fields fall back to default tuning.
func NewSystem(pool *pgxpool.Pool, ids domain.IDGenerator, clock Clock, logger *slog.Logger, cfg Config) *System {
	cfg = cfg.withDefaults()
	registry := NewRegistry()
	store := NewStore(pool, ids, cfg.LeaseDuration)
	live := newLiveJobs()
	return &System{
		queue:  NewQueue(store, registry, ids, clock, live),
		engine: NewEngine(store, registry, clock, ids, logger, cfg, live),
	}
}

// Queue is the handle callers inject to enqueue and query jobs.
func (s *System) Queue() *Queue { return s.queue }

// Start launches the worker pool.
func (s *System) Start(ctx context.Context) { s.engine.Start(ctx) }

// Shutdown stops the worker pool within ctx's deadline, leaving any
// still-running job for the reaper. It is called between the
// HTTP server's shutdown and the connection pool's close.
func (s *System) Shutdown(ctx context.Context) error { return s.engine.Shutdown(ctx) }

// Pause halts worker claiming of new jobs.
func (s *System) Pause() { s.engine.Pause() }

// Resume resumes worker claiming of new jobs.
func (s *System) Resume() { s.engine.Resume() }

// IsPaused reports whether workers are paused.
func (s *System) IsPaused() bool { return s.engine.IsPaused() }
