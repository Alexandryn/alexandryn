//go:build integration

package testutil_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// Variant B's own proof (backend-test-harness.md FR-3, T26): the
// composability hazard is two packages' integration-tagged TestMains
// resetting/migrating the same TEST_DATABASE_URL database concurrently.
// These tests prove EnsurePackageDatabase actually produces independent
// databases and is safe to call more than once — the mechanism T26-4/5/6
// wire into internal/testutil, internal/persistence/postgres, and
// cmd/server's own TestMains, in place of the -p 1 workaround. Shares
// this file's TestMain with isolation_integration_test.go (same package,
// same build tag — Go allows exactly one TestMain per test binary).

func adminDBFor(t *testing.T, url string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func databaseExists(t *testing.T, admin *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	err := admin.QueryRowContext(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("checking pg_database for %q: %v", name, err)
	}
	return exists
}

func TestEnsurePackageDatabase_TwoPkgNamesProduceIndependentDatabases(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")

	dbNameA, urlA, err := testutil.DerivePackageDatabaseURL(base, "proofa")
	if err != nil {
		t.Fatalf("DerivePackageDatabaseURL(proofa): %v", err)
	}
	dbNameB, urlB, err := testutil.DerivePackageDatabaseURL(base, "proofb")
	if err != nil {
		t.Fatalf("DerivePackageDatabaseURL(proofb): %v", err)
	}

	gotA, err := testutil.EnsurePackageDatabase(context.Background(), base, "proofa")
	if err != nil {
		t.Fatalf("EnsurePackageDatabase(proofa): %v", err)
	}
	if gotA != urlA {
		t.Fatalf("EnsurePackageDatabase(proofa) = %q, want %q", gotA, urlA)
	}

	gotB, err := testutil.EnsurePackageDatabase(context.Background(), base, "proofb")
	if err != nil {
		t.Fatalf("EnsurePackageDatabase(proofb): %v", err)
	}
	if gotB != urlB {
		t.Fatalf("EnsurePackageDatabase(proofb) = %q, want %q", gotB, urlB)
	}

	admin := adminDBFor(t, base)
	if !databaseExists(t, admin, dbNameA) {
		t.Fatalf("database %q does not exist after EnsurePackageDatabase", dbNameA)
	}
	if !databaseExists(t, admin, dbNameB) {
		t.Fatalf("database %q does not exist after EnsurePackageDatabase", dbNameB)
	}
}

func TestEnsurePackageDatabase_RepeatCallIsIdempotent(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")

	first, err := testutil.EnsurePackageDatabase(context.Background(), base, "proofrepeat")
	if err != nil {
		t.Fatalf("EnsurePackageDatabase (first call): %v", err)
	}

	second, err := testutil.EnsurePackageDatabase(context.Background(), base, "proofrepeat")
	if err != nil {
		t.Fatalf("EnsurePackageDatabase (second call): %v", err)
	}

	if first != second {
		t.Fatalf("second call returned %q, want the same URL as the first call %q", second, first)
	}
}
