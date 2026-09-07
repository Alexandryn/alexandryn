//go:build integration

package http_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.IntegrationTestMain(
		os.LookupEnv,
		testutil.WithPackageDatabase("transporthttp", os.Getenv, os.Setenv, os.Stderr, m.Run),
		os.Stderr,
	))
}

func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")

	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		_ = db.Close()
		t.Fatalf("reset schema: %v", err)
	}
	_ = db.Close()

	if err := postgres.Migrate(context.Background(), url); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	pool, err := postgres.NewPool(context.Background(), url, 12)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
