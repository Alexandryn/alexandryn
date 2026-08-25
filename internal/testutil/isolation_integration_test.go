//go:build integration

package testutil_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// T22 (tasks/plan.md Tier 4): backend-test-harness.md FR-3's own
// mechanisms, proven by using them — this spec's own Test strategy
// frames it exactly this way ("testing the test harness means proving
// its mechanisms work as advertised, using them"). Variant B (the
// cross-package composability hazard, proven in
// packagedb_integration_test.go and wired in below) is T26's.

// TestMain gives this package its own isolated database
// (EnsurePackageDatabase, FR-3 Variant B, T26-4) before reaching
// migration head once, here, before any test function in this file
// runs — the concrete mechanism behind the schema-at-head proof below:
// no test in this file calls Migrate itself.
func TestMain(m *testing.M) {
	os.Exit(testutil.IntegrationTestMain(os.LookupEnv, func() int {
		isolatedURL, err := testutil.EnsurePackageDatabase(context.Background(), os.Getenv("TEST_DATABASE_URL"), "testutil")
		if err != nil {
			fmt.Fprintf(os.Stderr, "harness setup: EnsurePackageDatabase: %v\n", err)
			return 1
		}
		os.Setenv("TEST_DATABASE_URL", isolatedURL)

		if err := postgres.Migrate(context.Background(), os.Getenv("TEST_DATABASE_URL")); err != nil {
			fmt.Fprintf(os.Stderr, "harness setup: Migrate: %v\n", err)
			return 1
		}
		return m.Run()
	}, os.Stderr))
}

func isolationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

const isolationFixtureTable = "testutil_isolation_fixture"

func ensureIsolationFixtureTable(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`CREATE TABLE IF NOT EXISTS `+isolationFixtureTable+` (id serial PRIMARY KEY, value text)`); err != nil {
		t.Fatalf("CREATE TABLE IF NOT EXISTS %s: %v", isolationFixtureTable, err)
	}
}

// FR-3 schema-at-head: this test never calls Migrate itself — TestMain
// already reached migration head as part of harness setup, above — and
// still finds goose's own tracking table populated. Proves the harness's
// setup step, not the test author, is responsible for reaching head.
func TestSchemaAtHead_HarnessSetupReachesItWithoutTheTestCallingMigrate(t *testing.T) {
	db := isolationDB(t)

	var exists bool
	err := db.QueryRowContext(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'goose_db_version')`,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("checking for goose_db_version: %v", err)
	}
	if !exists {
		t.Fatal("goose_db_version doesn't exist — migrations were never applied by harness setup")
	}
}

// FR-3 isolation, Variant A: two tests against the same table,
// deliberately run in the same `go test` invocation with real
// truncate-based teardown between them via t.Cleanup(testutil.TruncateTables)
// — the harness's own prescribed mechanism, not ad hoc per-test SQL.
// TestIsolationA runs first (Go runs a package's tests in source order
// unless a test opts into running concurrently, which
// scripts/check-integration-test-parallelism.sh forbids in any
// _integration_test.go file, this one included); TestIsolationB asserts
// zero rows exist at its own start, proving TestIsolationA's data didn't
// leak forward.
func TestIsolationA(t *testing.T) {
	db := isolationDB(t)
	ensureIsolationFixtureTable(t, db)
	t.Cleanup(func() {
		if err := testutil.TruncateTables(context.Background(), db, isolationFixtureTable); err != nil {
			t.Errorf("TruncateTables cleanup: %v", err)
		}
	})

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO `+isolationFixtureTable+` (value) VALUES ('from-a')`); err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM `+isolationFixtureTable).Scan(&count); err != nil {
		t.Fatalf("SELECT count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func TestIsolationB(t *testing.T) {
	db := isolationDB(t)
	ensureIsolationFixtureTable(t, db)
	t.Cleanup(func() {
		if err := testutil.TruncateTables(context.Background(), db, isolationFixtureTable); err != nil {
			t.Errorf("TruncateTables cleanup: %v", err)
		}
	})

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM `+isolationFixtureTable).Scan(&count); err != nil {
		t.Fatalf("SELECT count: %v", err)
	}
	if count != 0 {
		t.Fatalf("row count at start = %d, want 0 — TestIsolationA's data leaked forward, truncate-based teardown failed", count)
	}
}
