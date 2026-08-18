package postgres_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// FR-1: pool sizing is config-driven for MaxConns, fixed for MinConns —
// proven by inspecting the constructed pgxpool.Config, no real
// connection needed (backend-persistence.md's own test plan).
func TestPoolConfig_MaxConnsMatchesConfig(t *testing.T) {
	cfg, err := postgres.PoolConfig("postgres://user:pass@localhost:5432/alexandryn", 25)
	if err != nil {
		t.Fatalf("PoolConfig: %v", err)
	}
	if cfg.MaxConns != 25 {
		t.Fatalf("MaxConns = %d, want 25 (from the given DB_POOL_MAX_CONNS)", cfg.MaxConns)
	}
}

func TestPoolConfig_MinConnsAlwaysTwoRegardlessOfMaxConns(t *testing.T) {
	cases := []int{1, 10, 100}
	for _, maxConns := range cases {
		cfg, err := postgres.PoolConfig("postgres://user:pass@localhost:5432/alexandryn", maxConns)
		if err != nil {
			t.Fatalf("PoolConfig(maxConns=%d): %v", maxConns, err)
		}
		if cfg.MinConns != 2 {
			t.Fatalf("MinConns = %d, want 2 (fixed, no config key, DB_POOL_MAX_CONNS=%d)", cfg.MinConns, maxConns)
		}
	}
}

func TestPoolConfig_InvalidDatabaseURLErrors(t *testing.T) {
	_, err := postgres.PoolConfig("not a connection string", 10)
	if err == nil {
		t.Fatal("PoolConfig() error = nil, want an error for a malformed DATABASE_URL")
	}
}
