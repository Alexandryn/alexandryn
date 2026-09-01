package jobs

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// System is the job subsystem as one wired unit: the enqueue/query
// Queue that future job handlers use, and the worker pool that runs
// them. cmd/server constructs one against the shared pool at
// backend-service-lifecycle.md FR-1 step 6 and drives its lifecycle
// (FR-6, amended for phase 09).
type System struct {
	queue  *Queue
	engine *Engine
}

// NewSystem builds the subsystem over the process's shared connection
// pool. ids is the production UUID generator; clock is the real wall
// clock; cfg's zero fields fall back to the spec's placeholder tuning.
func NewSystem(pool *pgxpool.Pool, ids domain.IDGenerator, clock Clock, logger *slog.Logger, cfg Config) *System {
	cfg = cfg.withDefaults()
	registry := NewRegistry()
	store := NewStore(pool, ids, cfg.LeaseDuration)
	return &System{
		queue:  NewQueue(store, registry, ids, clock),
		engine: NewEngine(store, registry, clock, ids, logger, cfg),
	}
}

// Queue is the handle future phases inject to enqueue and query jobs.
func (s *System) Queue() *Queue { return s.queue }

// Start launches the worker pool (FR-1 step 6 / step 7).
func (s *System) Start(ctx context.Context) { s.engine.Start(ctx) }

// Shutdown stops the worker pool within ctx's deadline, leaving any
// still-running job for the reaper (FR-10). It is called between the
// HTTP server's shutdown and the connection pool's close (FR-6).
func (s *System) Shutdown(ctx context.Context) error { return s.engine.Shutdown(ctx) }
