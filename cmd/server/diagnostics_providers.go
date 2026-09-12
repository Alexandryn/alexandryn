package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/observability"
)

// poolStatsProvider adapts a live pgxpool to the diagnostics endpoint's
// PoolStatsProvider to expose connection pool metrics.
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
// diagnostics endpoint's QueueDepthProvider to report queue depth.
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
