//go:build integration

package postgres_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// tracedQuery is one call pgx actually sent, as captured by queryTracer —
// the literal SQL template plus the separate argument list.
type tracedQuery struct {
	SQL  string
	Args []any
}

// queryTracer is a pgx.QueryTracer that records every query pgx sends
// over a traced pool's connections to verify parameterized queries: only
// inspecting the literal SQL text pgx actually transmits (a placeholder
// at the hostile field's position, the hostile value only in Args, never
// interpolated into SQL) ensures queries are genuinely parameterized.
type queryTracer struct {
	mu      sync.Mutex
	queries []tracedQuery
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queries = append(t.queries, tracedQuery{SQL: data.SQL, Args: data.Args})
	return ctx
}

func (t *queryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (t *queryTracer) queriesContaining(substr string) []tracedQuery {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result []tracedQuery
	for _, q := range t.queries {
		if strings.Contains(q.SQL, substr) {
			result = append(result, q)
		}
	}
	return result
}

// tracedPool migrates a clean schema, then returns a real *pgxpool.Pool
// whose every query is captured by the returned *queryTracer.
func tracedPool(t *testing.T) (*pgxpool.Pool, *queryTracer) {
	t.Helper()
	db := testDB(t)
	resetSchema(t, db)
	url := os.Getenv("TEST_DATABASE_URL")
	if err := postgres.Migrate(context.Background(), url); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	tracer := &queryTracer{}
	cfg.ConnConfig.Tracer = tracer

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, tracer
}
