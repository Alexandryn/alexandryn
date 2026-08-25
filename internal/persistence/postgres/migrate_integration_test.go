//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// TestMain gives this package its own isolated database
// (testutil.WithPackageDatabase, backend-test-harness.md FR-3 Variant B,
// T26-5) before any test in this file runs.
func TestMain(m *testing.M) {
	os.Exit(testutil.IntegrationTestMain(os.LookupEnv, testutil.WithPackageDatabase("postgres", os.Getenv, os.Setenv, os.Stderr, m.Run), os.Stderr))
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// resetSchema gives each test a genuinely empty database, regardless of
// what an earlier run left behind — backend-test-harness.md FR-3's
// isolation requirement, applied here as a full schema reset rather than
// per-table truncation, since a migration test's whole point is what
// exists in the schema itself, including goose's own tracking table.
func resetSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("resetSchema: %v", err)
	}
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRowContext(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
		name,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("tableExists(%q): %v", name, err)
	}
	return exists
}

// FR-6: migrations apply to an empty database — phase 03's own named
// exit criterion — proven against the real embedded migration files via
// the real, production Migrate function, not a fixture.
func TestMigrate_AppliesToAnEmptyDatabase(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dsn := os.Getenv("TEST_DATABASE_URL")
	if err := postgres.Migrate(context.Background(), dsn); err != nil {
		t.Fatalf("Migrate() error = %v, want nil against an empty database", err)
	}

	if !tableExists(t, db, "goose_db_version") {
		t.Fatal("goose's own tracking table doesn't exist after a successful Migrate — schema didn't actually reach head")
	}
}

// FR-7: the next startup attempt after a failed, partial migration must
// also fail at the same step, not silently proceed — goose's own
// applied-migrations tracking table is what makes this true, proven here
// by actually causing a partial failure and retrying, not inferred from
// goose's documentation.
func TestPartialMigration_RefusedOnNextAttempt(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dir := t.TempDir()
	writeMigration(t, dir, "00001_ok.sql", "CREATE TABLE partial_ok (id int);")
	writeMigration(t, dir, "00002_broken.sql", "CRATE TABLE partial_bad (id int);") // deliberate typo

	up := func(ctx context.Context, db *sql.DB) error {
		goose.SetBaseFS(os.DirFS(dir))
		if err := goose.SetDialect("postgres"); err != nil {
			return err
		}
		return goose.UpContext(ctx, db, ".")
	}

	dsn := os.Getenv("TEST_DATABASE_URL")

	firstErr := postgres.RunMigrations(context.Background(), dsn, sql.Open, up)
	if firstErr == nil {
		t.Fatal("first RunMigrations() error = nil, want the broken migration to fail")
	}
	if !tableExists(t, db, "partial_ok") {
		t.Fatal("the migration before the broken one never applied — nothing to be \"partial\" about")
	}
	if tableExists(t, db, "partial_bad") {
		t.Fatal("the broken migration's table exists — it should have failed before creating anything")
	}

	secondErr := postgres.RunMigrations(context.Background(), dsn, sql.Open, up)
	if secondErr == nil {
		t.Fatal("second RunMigrations() error = nil — a partial migration must be refused again on the next attempt, not silently skipped")
	}
	if !tableExists(t, db, "partial_ok") {
		t.Fatal("the first migration's table disappeared between attempts")
	}
	if tableExists(t, db, "partial_bad") {
		t.Fatal("the broken migration applied on the second attempt — it should still fail the same way")
	}
}

func writeMigration(t *testing.T, dir, name, upSQL string) {
	t.Helper()
	content := "-- +goose Up\n" + upSQL + "\n\n-- +goose Down\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeMigration(%q): %v", name, err)
	}
}
