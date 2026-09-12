//go:build integration

package jobs_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// TestMain gives this package its own isolated database
// before any test runs, and fails loudly when TEST_DATABASE_URL is unset.
func TestMain(m *testing.M) {
	os.Exit(testutil.IntegrationTestMain(
		os.LookupEnv,
		testutil.WithPackageDatabase("jobs", os.Getenv, os.Setenv, os.Stderr, m.Run),
		os.Stderr,
	))
}

// migratedPool resets the schema, migrates to head, and returns a real
// pool — the same shape internal/persistence/postgres's own schemaTestPool
// uses, so each test starts from a genuinely empty jobs table.
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

// baseTime is a fixed instant every time-driven store test computes
// offsets from — no real clock, no real sleep.
var baseTime = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// seqIDs returns a domain.IDGenerator yielding id-1, id-2, ... — enough
// for any single test's claims and reclaims.
func seqIDs() *testutil.FakeIDGenerator {
	ids := make([]string, 0, 256)
	for i := 1; i <= 256; i++ {
		ids = append(ids, "id-"+itoa(i))
	}
	return testutil.NewFakeIDGenerator(ids...)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
