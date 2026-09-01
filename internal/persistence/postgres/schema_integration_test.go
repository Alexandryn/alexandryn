//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

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

// backend-source-adapter.md FR-1/FR-6/FR-13: migration 00005 adds
// source config, encrypted-credential, and health-state columns to the
// phase-02 `sources` table.
func TestSchema_Phase08SourceColumnsExist(t *testing.T) {
	pool := schemaTestPool(t)

	// A full phase-08 opds source row round-trips through every new column.
	mustExecPool(t, pool, `INSERT INTO sources
		(id, label, can_list, can_search, can_download, kind,
		 config_base_url, credential_ciphertext, credential_nonce,
		 health_status, health_detail, health_checked_at, search_link_url)
		VALUES ('src-p8', 'Personal OPDS', true, true, true, 'opds',
		 'https://opds.example.org/catalog', '\xdeadbeef', '\xcafe',
		 'unreachable', 'auth-rejected', now(), 'https://opds.example.org/search')`)

	var status, detail, url string
	var ct []byte
	err := pool.QueryRow(context.Background(),
		`SELECT health_status, health_detail, search_link_url, credential_ciphertext
		 FROM sources WHERE id = 'src-p8'`).Scan(&status, &detail, &url, &ct)
	if err != nil {
		t.Fatalf("scan phase-08 columns: %v", err)
	}
	if status != "unreachable" || detail != "auth-rejected" {
		t.Fatalf("health = %q/%q, want unreachable/auth-rejected", status, detail)
	}
	if url != "https://opds.example.org/search" || len(ct) != 4 {
		t.Fatalf("search_link_url=%q ciphertext=%d bytes, unexpected", url, len(ct))
	}
}

// domain-source.md FR-1: the schema does not constrain `kind` — an
// empty string (pre-phase-08 rows) and any other string are both
// accepted. The closed local-folder/opds vocabulary lives in the HTTP
// handler, not here.
func TestSchema_SourceKindNotConstrained(t *testing.T) {
	pool := schemaTestPool(t)
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('s-lf', 'L', true, false, true, 'local-folder')")
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download) VALUES ('s-empty', 'L', true, false, true)")
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('s-legacy', 'L', true, false, true, 'kind-a')")
}

// FR-6: health_detail is a closed vocabulary; NULL is legal.
func TestSchema_SourceHealthDetailConstrained(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO sources (id, label, can_list, can_search, can_download, kind, health_status, health_detail)
		VALUES ('s-hd', 'L', true, false, true, 'opds', 'unreachable', 'kaboom')`)
	assertCheckViolation(t, err)
}

// FR-1: a credential belongs to an opds source only, and its two BYTEA
// columns are written and cleared as a pair.
func TestSchema_SourceCredentialConstraints(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `INSERT INTO sources (id, label, can_list, can_search, can_download, kind, credential_ciphertext, credential_nonce)
		VALUES ('s-cred-lf', 'L', true, false, true, 'local-folder', '\xaa', '\xbb')`)
	assertCheckViolation(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO sources (id, label, can_list, can_search, can_download, kind, credential_ciphertext)
		VALUES ('s-cred-half', 'L', true, false, true, 'opds', '\xaa')`)
	assertCheckViolation(t, err)
}

// backend-persistence.md acceptance criterion: a down migration is
// reversible. Migrate to head, roll 00005 back via goose, and confirm
// the phase-02 `sources` shape is restored (the new column is gone) and
// then re-applies cleanly.
func TestSchema_Phase08MigrationIsReversible(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)
	if err := postgres.Migrate(context.Background(), os.Getenv("TEST_DATABASE_URL")); err != nil {
		t.Fatalf("Migrate up: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT config_base_url FROM sources LIMIT 1"); err != nil {
		t.Fatalf("config_base_url should exist after up: %v", err)
	}

	goose.SetBaseFS(os.DirFS("migrations"))
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("SetDialect: %v", err)
	}
	// Roll back to version 4 (before 00005) explicitly rather than a
	// single step, so this test stays correct as later migrations are
	// added on top of head.
	if err := goose.DownToContext(context.Background(), db, ".", 4); err != nil {
		t.Fatalf("goose down to 00004: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), "SELECT config_base_url FROM sources LIMIT 1"); err == nil {
		t.Fatal("config_base_url still present after down migration 00005")
	}
	if _, err := db.ExecContext(context.Background(), "SELECT label, can_list FROM sources LIMIT 1"); err != nil {
		t.Fatalf("phase-02 sources columns should survive the down migration: %v", err)
	}

	if err := goose.UpContext(context.Background(), db, "."); err != nil {
		t.Fatalf("goose re-up 00005: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT config_base_url FROM sources LIMIT 1"); err != nil {
		t.Fatalf("config_base_url should exist again after re-up: %v", err)
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

// Migration 00008 (phase 11): reading_progress.epoch and
// bookmarks/highlights.created_at exist, with their DEFAULT applied to
// rows inserted without them (domain-reading.md FR-6 as amended;
// reading-data-export.md FR-4).
func TestSchema_Phase11ReaderColumns(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language) VALUES ('edition-1', 'work-1', 'en')")
	mustExecPool(t, pool, "INSERT INTO reading_progress (id, work_id, percentage, device_id, observed_at) VALUES ('progress-1', 'work-1', 0.5, 'device-1', now())")
	mustExecPool(t, pool, "INSERT INTO bookmarks (id, edition_id, position) VALUES ('bookmark-1', 'edition-1', 'epubcfi(/6/4!/4)')")
	mustExecPool(t, pool, "INSERT INTO highlights (id, edition_id, start_position, end_position) VALUES ('highlight-1', 'edition-1', 'epubcfi(/6/4!/4/1:0)', 'epubcfi(/6/4!/4/1:9)')")

	var epoch int64
	if err := pool.QueryRow(ctx, "SELECT epoch FROM reading_progress WHERE id = 'progress-1'").Scan(&epoch); err != nil {
		t.Fatalf("select epoch: %v", err)
	}
	if epoch != 0 {
		t.Fatalf("epoch = %d, want 0 (the DEFAULT)", epoch)
	}

	var bookmarkCreated, highlightCreated time.Time
	if err := pool.QueryRow(ctx, "SELECT created_at FROM bookmarks WHERE id = 'bookmark-1'").Scan(&bookmarkCreated); err != nil {
		t.Fatalf("select bookmarks.created_at: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT created_at FROM highlights WHERE id = 'highlight-1'").Scan(&highlightCreated); err != nil {
		t.Fatalf("select highlights.created_at: %v", err)
	}
	if bookmarkCreated.IsZero() || highlightCreated.IsZero() {
		t.Fatalf("created_at DEFAULT not applied: bookmark=%v highlight=%v", bookmarkCreated, highlightCreated)
	}
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

func assertCheckViolation(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a check-constraint violation, got nil error")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("error = %v, want SQLSTATE 23514 (check_violation)", err)
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
