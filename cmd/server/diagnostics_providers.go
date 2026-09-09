package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/observability"
)

// poolStatsProvider adapts a live pgxpool to the diagnostics endpoint's
// PoolStatsProvider. Without this wiring GET /api/v1/diagnostics reports a
// zeroed db_pool (audit 0016 #295).
func poolStatsProvider(pool *pgxpool.Pool) observability.PoolStatsProvider {
	return func() observability.PoolStats {
		s := pool.Stat()
		return observability.PoolStats{
			AcquiredConns: s.AcquiredConns(),
			IdleConns:     s.IdleConns(),
			TotalConns:    s.TotalConns(),
			MaxConns:      s.MaxConns(),
		}
	}
}

// queueDepthProvider adapts the job queue's per-state counts to the
// diagnostics endpoint's QueueDepthProvider. Without this wiring
// GET /api/v1/diagnostics reports an empty queue_depth (audit 0016 #295).
func queueDepthProvider(count func(context.Context) (map[jobs.State]int, error)) observability.QueueDepthProvider {
	return func(ctx context.Context) (map[string]int, error) {
		counts, err := count(ctx)
		if err != nil {
			return nil, err
		}
		out := make(map[string]int, len(counts))
		for state, n := range counts {
			out[string(state)] = n
		}
		return out, nil
	}
}
