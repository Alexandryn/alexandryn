//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// backend-import-pipeline.md FR-3: migration 00007 adds the `import_candidates` table
// with the columns and indexes the import pipeline reads and writes.
func TestSchema_ImportCandidatesTableExistsAfterMigration(t *testing.T) {
	pool := schemaTestPool(t)

	mustExecPool(t, pool, `INSERT INTO sources (id, label, kind, can_list, can_search, can_download)
		VALUES ('src-test-import-1', 'Test Source', 'local-folder', true, true, true) ON CONFLICT DO NOTHING`)

	// A full import_candidate row round-trips through every column.
	mustExecPool(t, pool, `INSERT INTO import_candidates
		(id, source_id, file_reference, status, extracted_metadata,
		 match_candidates, job_id, last_error, created_at, updated_at)
		VALUES ('cand-1', 'src-test-import-1', '{"id":"ref-1","format":"epub"}'::jsonb, 'queued',
		 '{"title":"Test Book","authors":["Author One"]}'::jsonb,
		 '[{"type":"open_library_work","confidence":"high","title":"Test Book"}]'::jsonb,
		 'job-1', 'some failure', now(), now())`)

	var status, jobID, lastError string
	err := pool.QueryRow(context.Background(),
		`SELECT status, job_id, last_error
		 FROM import_candidates WHERE id = 'cand-1'`).Scan(&status, &jobID, &lastError)
	if err != nil {
		t.Fatalf("scan import_candidates columns: %v", err)
	}
	if status != "queued" || jobID != "job-1" || lastError != "some failure" {
		t.Fatalf("row = %q/%q/%q, unexpected", status, jobID, lastError)
	}
}

// FR-3: `status` is a closed six-value vocabulary.
func TestSchema_ImportCandidatesStatusConstrained(t *testing.T) {
	pool := schemaTestPool(t)

	mustExecPool(t, pool, `INSERT INTO sources (id, label, kind, can_list, can_search, can_download)
		VALUES ('src-test-import-2', 'Test Source', 'local-folder', true, true, true) ON CONFLICT DO NOTHING`)

	_, err := pool.Exec(context.Background(), `INSERT INTO import_candidates
		(id, source_id, file_reference, status, created_at, updated_at)
		VALUES ('cand-bad', 'src-test-import-2', '{"id":"1","format":"epub"}'::jsonb, 'in-flight', now(), now())`)
	assertCheckViolation(t, err)

	for _, s := range []string{"queued", "pending", "auto_imported", "confirmed", "rejected", "failed"} {
		mustExecPool(t, pool, `INSERT INTO import_candidates
			(id, source_id, file_reference, status, created_at, updated_at)
			VALUES ('cand-`+s+`', 'src-test-import-2', '{"id":"1","format":"epub"}'::jsonb, '`+s+`', now(), now())`)
	}
}

// FR-3: composite index on (source_id, status) exists for dedup and filtering.
func TestSchema_ImportCandidatesIndexExists(t *testing.T) {
	pool := schemaTestPool(t)

	var exists bool
	if err := pool.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'import_candidates' AND indexname = 'import_candidates_source_status_idx')",
	).Scan(&exists); err != nil {
		t.Fatalf("checking index import_candidates_source_status_idx: %v", err)
	}
	if !exists {
		t.Fatal("index import_candidates_source_status_idx does not exist")
	}
}

// backend-persistence.md acceptance criterion: the down migration is reversible and re-appliable.
func TestSchema_ImportCandidatesMigrationIsReversible(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)
	if err := postgres.Migrate(context.Background(), os.Getenv("TEST_DATABASE_URL")); err != nil {
		t.Fatalf("Migrate up: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM import_candidates LIMIT 1"); err != nil {
		t.Fatalf("import_candidates table should exist after up: %v", err)
	}

	goose.SetBaseFS(os.DirFS("migrations"))
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("SetDialect: %v", err)
	}
	if err := goose.DownToContext(context.Background(), db, ".", 6); err != nil {
		t.Fatalf("goose down to 00006: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM import_candidates LIMIT 1"); err == nil {
		t.Fatal("import_candidates table still present after down migration 00007")
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM jobs LIMIT 1"); err != nil {
		t.Fatalf("earlier migrations' tables should survive the down migration: %v", err)
	}

	if err := goose.UpContext(context.Background(), db, "."); err != nil {
		t.Fatalf("goose re-up 00007: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM import_candidates LIMIT 1"); err != nil {
		t.Fatalf("import_candidates table should exist again after re-up: %v", err)
	}
}
