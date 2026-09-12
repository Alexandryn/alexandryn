// Package postgres implements Alexandryn's repository interfaces
// against PostgreSQL via pgx, and owns the connection pool and migration
// runner's construction.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MinConns is fixed, not configurable: a round-number floor that keeps a
// couple of connections warm at idle, not a capacity ceiling — it has no
// DB_POOL_MAX_CONNS-style config key because it doesn't need
// per-deployment tuning the way MaxConns does.
const MinConns = 2

// PoolConfig builds a *pgxpool.Config for a single-process pool, sized
// MinConns/maxConns, without opening a real connection — inspectable in a
// test with no real Postgres. maxConns comes from config.Config.DBPoolMaxConns.
func PoolConfig(databaseURL string, maxConns int) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse DATABASE_URL: %w", err)
	}
	cfg.MinConns = MinConns
	cfg.MaxConns = int32(maxConns)
	return cfg, nil
}

// NewPool constructs and opens the *pgxpool.Pool this process uses,
// per PoolConfig's sizing. Constructed once in server startup and passed to
// repository constructors — never a package-level global.
func NewPool(ctx context.Context, databaseURL string, maxConns int) (*pgxpool.Pool, error) {
	cfg, err := PoolConfig(databaseURL, maxConns)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}
