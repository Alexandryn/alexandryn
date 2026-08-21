//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// backend-persistence.md FR-2: all 11 aggregate tables (normalized into
// more physical tables where an aggregate owns a collection) plus the
// outbox table (ADR 0021) exist after migrating to head.
func TestSchema_AllPhase02TablesExistAfterMigration(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)
	if err := postgres.Migrate(context.Background(), os.Getenv("TEST_DATABASE_URL")); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Every query is a complete, static string literal — no dynamic
	// identifier building, matching exactly how every real repository
	// method's SQL looks (a table name is always compile-time-known in
	// this codebase, never computed; backend-persistence.md FR-3).
	queries := []string{
		"SELECT 1 FROM works LIMIT 1",
		"SELECT 1 FROM work_authors LIMIT 1",
		"SELECT 1 FROM work_subjects LIMIT 1",
		"SELECT 1 FROM work_external_references LIMIT 1",
		"SELECT 1 FROM work_contains LIMIT 1",
		"SELECT 1 FROM authors LIMIT 1",
		"SELECT 1 FROM author_external_references LIMIT 1",
		"SELECT 1 FROM editions LIMIT 1",
		"SELECT 1 FROM edition_external_references LIMIT 1",
		"SELECT 1 FROM library_entries LIMIT 1",
		"SELECT 1 FROM collections LIMIT 1",
		"SELECT 1 FROM collection_members LIMIT 1",
		"SELECT 1 FROM sources LIMIT 1",
		"SELECT 1 FROM source_offerings LIMIT 1",
		"SELECT 1 FROM reading_progress LIMIT 1",
		"SELECT 1 FROM bookmarks LIMIT 1",
		"SELECT 1 FROM highlights LIMIT 1",
		"SELECT 1 FROM reading_preferences LIMIT 1",
		"SELECT 1 FROM outbox LIMIT 1",
	}
	for _, query := range queries {
		if _, err := db.ExecContext(context.Background(), query); err != nil {
			t.Errorf("query %q: %v", query, err)
		}
	}
}

// domain-library.md FR-7: at most one LibraryEntry per Edition — proven
// against the real UNIQUE constraint, not just application logic (R6's
// own concurrency proof depends on this constraint actually existing).
func TestSchema_LibraryEntriesUniqueByEdition(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language) VALUES ('edition-1', 'work-1', 'en')")
	mustExecPool(t, pool, "INSERT INTO library_entries (id, edition_id, added_at) VALUES ('entry-1', 'edition-1', now())")

	_, err := pool.Exec(ctx, "INSERT INTO library_entries (id, edition_id, added_at) VALUES ('entry-2', 'edition-1', now())")
	assertUniqueViolation(t, err)
}

// domain-source.md FR-2: unique by (Source, Edition, Format).
func TestSchema_SourceOfferingsUniqueBySourceEditionFormat(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language) VALUES ('edition-1', 'work-1', 'en')")
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download) VALUES ('source-1', 'Src', true, false, true)")
	mustExecPool(t, pool, `INSERT INTO source_offerings (id, source_id, edition_id, file_reference_id, file_reference_format, observed_at)
		VALUES ('offering-1', 'source-1', 'edition-1', 'ref-1', 'epub', now())`)

	// Same (source, edition, format) — must collide.
	_, err := pool.Exec(ctx, `INSERT INTO source_offerings (id, source_id, edition_id, file_reference_id, file_reference_format, observed_at)
		VALUES ('offering-2', 'source-1', 'edition-1', 'ref-2', 'epub', now())`)
	assertUniqueViolation(t, err)

	// Same (source, edition), different format — must succeed (two rows).
	if _, err := pool.Exec(ctx, `INSERT INTO source_offerings (id, source_id, edition_id, file_reference_id, file_reference_format, observed_at)
		VALUES ('offering-3', 'source-1', 'edition-1', 'ref-3', 'pdf', now())`); err != nil {
		t.Fatalf("different-format offering should succeed: %v", err)
	}
}

// domain-reading.md FR-1: at most one ReadingProgress per Work.
func TestSchema_ReadingProgressUniqueByWork(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO reading_progress (id, work_id, percentage, device_id, observed_at) VALUES ('progress-1', 'work-1', 0.5, 'device-1', now())")

	_, err := pool.Exec(ctx, "INSERT INTO reading_progress (id, work_id, percentage, device_id, observed_at) VALUES ('progress-2', 'work-1', 0.7, 'device-2', now())")
	assertUniqueViolation(t, err)
}

// domain-bibliographic.md FR-8: an Edition cannot exist without a real
// parent Work — the FK constraint is this table's own enforcement of
// the same guarantee the Go type gives structurally.
func TestSchema_EditionsRequireARealWork(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, "INSERT INTO editions (id, work_id, language) VALUES ('edition-1', 'nonexistent-work', 'en')")
	if err == nil {
		t.Fatal("INSERT with a nonexistent work_id succeeded, want a foreign-key violation")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("error = %v, want SQLSTATE 23503 (foreign_key_violation), not some other failure", err)
	}
}

// backend-persistence.md's own required acceptance criterion: a
// migration applied to a database with existing rows succeeds and
// leaves those rows intact. Migrating twice (the second call has
// nothing pending) with real data seeded between the two calls proves
// the mechanism startup relies on every time the server starts.
func TestSchema_ReMigratingLeavesExistingRowsIntact(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)
	url := os.Getenv("TEST_DATABASE_URL")
	if err := postgres.Migrate(context.Background(), url); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	pool, err := postgres.NewPool(context.Background(), url, 5)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Populated Before Re-Migrate')")

	if err := postgres.Migrate(context.Background(), url); err != nil {
		t.Fatalf("second Migrate (no pending migrations): %v", err)
	}

	var title string
	scanErr := pool.QueryRow(context.Background(), "SELECT title FROM works WHERE id = 'work-1'").Scan(&title)
	if scanErr != nil {
		t.Fatalf("row did not survive re-migration: %v", scanErr)
	}
	if title != "Populated Before Re-Migrate" {
		t.Fatalf("title = %q, want unchanged", title)
	}
}

func assertUniqueViolation(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a unique-constraint violation, got nil error")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want SQLSTATE 23505 (unique_violation)", err)
	}
}

// schemaTestPool migrates a clean schema, then returns a real
// *pgxpool.Pool for the test's own inserts.
func schemaTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	db := testDB(t)
	resetSchema(t, db)
	url := os.Getenv("TEST_DATABASE_URL")
	if err := postgres.Migrate(context.Background(), url); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	pool, err := postgres.NewPool(context.Background(), url, 5)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func mustExecPool(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatalf("Exec(%q): %v", sql, err)
	}
}
