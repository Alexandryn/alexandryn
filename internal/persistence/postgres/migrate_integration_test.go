//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// TestMain gives this package its own isolated database before any test in this file runs.
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

// resetSchema gives each test an empty database — a full schema reset
// rather than per-table truncation, since a migration test's whole point is what
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

// Migrations apply cleanly to an empty database — proven against the real
// embedded migration files via the real, production Migrate function.
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

// The next startup attempt after a failed, partial migration must
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

// When a failed migration is repaired, the subsequent startup attempt must
// successfully apply it without losing prior migration data.
func TestPartialMigration_RecoverableAfterFix(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dir := t.TempDir()
	writeMigration(t, dir, "00001_ok.sql", "CREATE TABLE recover_step1 (id int PRIMARY KEY, val text);")
	writeMigration(t, dir, "00002_bad.sql", "CRATE TABLE recover_step2 (id int);") // syntax error

	up := func(ctx context.Context, db *sql.DB) error {
		goose.SetBaseFS(os.DirFS(dir))
		if err := goose.SetDialect("postgres"); err != nil {
			return err
		}
		return goose.UpContext(ctx, db, ".")
	}

	dsn := os.Getenv("TEST_DATABASE_URL")

	if err := postgres.RunMigrations(context.Background(), dsn, sql.Open, up); err == nil {
		t.Fatal("expected first run to fail")
	}

	// Insert row into step1 table to ensure data survives migration recovery.
	if _, err := db.ExecContext(context.Background(), "INSERT INTO recover_step1 (id, val) VALUES (1, 'persisted');"); err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Fix migration 2 by replacing it with valid DDL.
	writeMigration(t, dir, "00002_bad.sql", "CREATE TABLE recover_step2 (id int PRIMARY KEY, ref_id int REFERENCES recover_step1(id));")

	// Retry migrations
	if err := postgres.RunMigrations(context.Background(), dsn, sql.Open, up); err != nil {
		t.Fatalf("RunMigrations() after repair error = %v, want nil", err)
	}

	if !tableExists(t, db, "recover_step1") || !tableExists(t, db, "recover_step2") {
		t.Fatal("expected both tables to exist after repaired migration")
	}

	var val string
	if err := db.QueryRowContext(context.Background(), "SELECT val FROM recover_step1 WHERE id = 1").Scan(&val); err != nil || val != "persisted" {
		t.Fatalf("prior data lost or corrupted: err=%v, val=%q", err, val)
	}
}

// Running Migrate multiple times on an up-to-date database must be idempotent and safe.
func TestMigrate_IdempotentOnRepeat(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dsn := os.Getenv("TEST_DATABASE_URL")
	ctx := context.Background()

	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("first Migrate() failed: %v", err)
	}

	v1, err := postgres.CurrentVersion(ctx, dsn)
	if err != nil {
		t.Fatalf("CurrentVersion() failed: %v", err)
	}

	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("second Migrate() failed: %v", err)
	}

	v2, err := postgres.CurrentVersion(ctx, dsn)
	if err != nil {
		t.Fatalf("CurrentVersion() failed: %v", err)
	}

	if v1 != v2 {
		t.Fatalf("version changed on second run: v1=%d, v2=%d", v1, v2)
	}
}

// Upgrading an existing database with pre-populated rows from earlier migrations
// must preserve all existing records and successfully apply subsequent migrations.
func TestMigrate_PreservesExistingRecords(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dsn := os.Getenv("TEST_DATABASE_URL")
	ctx := context.Background()

	// Apply migrations up to Phase 2 schema (version 2).
	if err := postgres.MigrateTo(ctx, dsn, 2); err != nil {
		t.Fatalf("MigrateTo(2) error: %v", err)
	}

	v, err := postgres.CurrentVersion(ctx, dsn)
	if err != nil || v != 2 {
		t.Fatalf("expected version 2, got version %d (err: %v)", v, err)
	}

	// Seed domain data representing an active installation.
	queries := []string{
		`INSERT INTO works (id, title, subtitle, original_language)
		 VALUES ('wrk_001', 'Dune', 'Chronicles', 'en');`,

		`INSERT INTO authors (id, name)
		 VALUES ('aut_001', 'Frank Herbert');`,

		`INSERT INTO work_authors (work_id, author_id)
		 VALUES ('wrk_001', 'aut_001');`,

		`INSERT INTO work_subjects (work_id, subject)
		 VALUES ('wrk_001', 'Science Fiction');`,

		`INSERT INTO editions (id, work_id, language, publisher, publication_year)
		 VALUES ('edn_001', 'wrk_001', 'en', 'Chilton Books', 1965);`,

		`INSERT INTO library_entries (id, edition_id, added_at)
		 VALUES ('ent_001', 'edn_001', now());`,

		`INSERT INTO collections (id, name)
		 VALUES ('col_001', 'Classics');`,

		`INSERT INTO collection_members (collection_id, work_id, added_at)
		 VALUES ('col_001', 'wrk_001', now());`,

		`INSERT INTO sources (id, label, can_list, can_search, can_download, kind)
		 VALUES ('src_001', 'Local Books', true, false, true, 'local_folder');`,

		`INSERT INTO source_offerings (id, source_id, edition_id, file_reference_id, file_reference_format, file_reference_size_bytes, observed_at)
		 VALUES ('off_001', 'src_001', 'edn_001', 'dune.epub', 'EPUB', 1048576, now());`,

		`INSERT INTO reading_progress (id, work_id, percentage, precise_position_edition_id, precise_position_value, device_id, observed_at)
		 VALUES ('prog_001', 'wrk_001', 45.5, 'edn_001', 'cfi(/6/4)', 'dev_001', now());`,

		`INSERT INTO bookmarks (id, edition_id, position, label)
		 VALUES ('bmk_001', 'edn_001', 'cfi(/6/12)', 'Favorite passage');`,

		`INSERT INTO highlights (id, edition_id, start_position, end_position, note, category)
		 VALUES ('hlt_001', 'edn_001', 'cfi(/6/14:0)', 'cfi(/6/14:20)', 'Fear is the mind killer', 'quote');`,

		`INSERT INTO reading_preferences (device_id, settings)
		 VALUES ('dev_001', '{"theme": "dark", "fontSize": 18}'::jsonb);`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("failed seeding initial data with %q: %v", q, err)
		}
	}

	// Upgrade all the way to latest migration (version 14).
	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate() forward upgrade failed: %v", err)
	}

	finalVersion, err := postgres.CurrentVersion(ctx, dsn)
	if err != nil || finalVersion < 14 {
		t.Fatalf("expected version >= 14, got %d (err: %v)", finalVersion, err)
	}

	// Verify all pre-existing records survived intact with newly added schema attributes populated.
	var title string
	if err := db.QueryRowContext(ctx, "SELECT title FROM works WHERE id = 'wrk_001'").Scan(&title); err != nil || title != "Dune" {
		t.Fatalf("work row corrupted: %v", err)
	}

	var authorName string
	if err := db.QueryRowContext(ctx, "SELECT name FROM authors WHERE id = 'aut_001'").Scan(&authorName); err != nil || authorName != "Frank Herbert" {
		t.Fatalf("author row corrupted: %v", err)
	}

	var progressPct float64
	var epoch int64
	var syncSeq sql.NullInt64
	err = db.QueryRowContext(ctx, "SELECT percentage, epoch, sync_sequence FROM reading_progress WHERE id = 'prog_001'").Scan(&progressPct, &epoch, &syncSeq)
	if err != nil {
		t.Fatalf("reading_progress lookup failed: %v", err)
	}
	if progressPct != 45.5 {
		t.Fatalf("reading_progress percentage changed: %v", progressPct)
	}
	if epoch != 0 {
		t.Fatalf("reading_progress epoch default incorrect: %v", epoch)
	}
	if !syncSeq.Valid || syncSeq.Int64 <= 0 {
		t.Fatalf("reading_progress sync_sequence backfill failed: valid=%v, seq=%v", syncSeq.Valid, syncSeq.Int64)
	}

	var bmkCreated, hltCreated time.Time
	if err := db.QueryRowContext(ctx, "SELECT created_at FROM bookmarks WHERE id = 'bmk_001'").Scan(&bmkCreated); err != nil || bmkCreated.IsZero() {
		t.Fatalf("bookmark created_at column failed to populate: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT created_at FROM highlights WHERE id = 'hlt_001'").Scan(&hltCreated); err != nil || hltCreated.IsZero() {
		t.Fatalf("highlight created_at column failed to populate: %v", err)
	}

	// Verify new tables created in later migrations exist and canonical data is present.
	var defaultLibName string
	if err := db.QueryRowContext(ctx, "SELECT name FROM libraries WHERE id = '00000000-0000-0000-0000-000000000001'").Scan(&defaultLibName); err != nil || defaultLibName != "Default Library" {
		t.Fatalf("default library seed missing: %v", err)
	}
}

func writeMigration(t *testing.T, dir, name, upSQL string) {
	t.Helper()
	content := "-- +goose Up\n" + upSQL + "\n\n-- +goose Down\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeMigration(%q): %v", name, err)
	}
}
