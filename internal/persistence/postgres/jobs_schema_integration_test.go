//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// Migration 00006 adds the `jobs` table with
// the columns the store (internal/jobs) reads and writes.
func TestSchema_JobsTableExistsAfterMigration(t *testing.T) {
	pool := schemaTestPool(t)

	// A full job row round-trips through every column the store uses.
	mustExecPool(t, pool, `INSERT INTO jobs
		(id, kind, payload, status, attempts, max_attempts,
		 available_at, locked_until, lease_token, locked_by,
		 last_error, progress, created_at, updated_at, completed_at)
		VALUES ('job-1', 'synthetic', '{"n":1}'::jsonb, 'running', 1, 5,
		 now(), now() + interval '60 seconds', 'lease-abc', 'worker-1',
		 'boom', '{"current":3,"total":40}'::jsonb, now(), now(), NULL)`)

	var kind, status, leaseToken string
	var attempts, maxAttempts int
	err := pool.QueryRow(context.Background(),
		`SELECT kind, status, attempts, max_attempts, lease_token
		 FROM jobs WHERE id = 'job-1'`).Scan(&kind, &status, &attempts, &maxAttempts, &leaseToken)
	if err != nil {
		t.Fatalf("scan jobs columns: %v", err)
	}
	if kind != "synthetic" || status != "running" || attempts != 1 || maxAttempts != 5 || leaseToken != "lease-abc" {
		t.Fatalf("row = %q/%q/%d/%d/%q, unexpected", kind, status, attempts, maxAttempts, leaseToken)
	}
}

// `status` is a closed vocabulary.
func TestSchema_JobsStatusConstrained(t *testing.T) {
	pool := schemaTestPool(t)

	_, err := pool.Exec(context.Background(), `INSERT INTO jobs
		(id, kind, payload, status, max_attempts, available_at, created_at, updated_at)
		VALUES ('job-bad', 'k', '{}'::jsonb, 'in-progress', 3, now(), now(), now())`)
	assertCheckViolation(t, err)

	for _, s := range []string{"queued", "running", "retrying", "completed", "dead_letter"} {
		mustExecPool(t, pool, `INSERT INTO jobs
			(id, kind, payload, status, max_attempts, available_at, created_at, updated_at)
			VALUES ('job-`+s+`', 'k', '{}'::jsonb, '`+s+`', 3, now(), now(), now())`)
	}
}

// Attempts is non-negative and max_attempts is at least 1.
func TestSchema_JobsAttemptBoundsConstrained(t *testing.T) {
	pool := schemaTestPool(t)

	_, err := pool.Exec(context.Background(), `INSERT INTO jobs
		(id, kind, payload, status, attempts, max_attempts, available_at, created_at, updated_at)
		VALUES ('job-neg', 'k', '{}'::jsonb, 'queued', -1, 3, now(), now(), now())`)
	assertCheckViolation(t, err)

	_, err = pool.Exec(context.Background(), `INSERT INTO jobs
		(id, kind, payload, status, attempts, max_attempts, available_at, created_at, updated_at)
		VALUES ('job-zero-max', 'k', '{}'::jsonb, 'queued', 0, 0, now(), now(), now())`)
	assertCheckViolation(t, err)
}

// The claim query and the reaper sweep each have a supporting index.
func TestSchema_JobsClaimAndReaperIndexesExist(t *testing.T) {
	pool := schemaTestPool(t)

	for _, idx := range []string{"jobs_claim_idx", "jobs_reaper_idx"} {
		var exists bool
		if err := pool.QueryRow(context.Background(),
			"SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'jobs' AND indexname = $1)", idx,
		).Scan(&exists); err != nil {
			t.Fatalf("checking index %s: %v", idx, err)
		}
		if !exists {
			t.Fatalf("index %s does not exist", idx)
		}
	}
}

// Verifies that the down migration is
// reversible and re-appliable.
func TestSchema_JobsMigrationIsReversible(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)
	if err := postgres.Migrate(context.Background(), os.Getenv("TEST_DATABASE_URL")); err != nil {
		t.Fatalf("Migrate up: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM jobs LIMIT 1"); err != nil {
		t.Fatalf("jobs table should exist after up: %v", err)
	}

	goose.SetBaseFS(os.DirFS("migrations"))
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("SetDialect: %v", err)
	}
	if err := goose.DownToContext(context.Background(), db, ".", 5); err != nil {
		t.Fatalf("goose down to 00005: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM jobs LIMIT 1"); err == nil {
		t.Fatal("jobs table still present after down migration 00006")
	}
	if _, err := db.ExecContext(context.Background(), "SELECT label FROM sources LIMIT 1"); err != nil {
		t.Fatalf("earlier migrations' tables should survive the down migration: %v", err)
	}

	if err := goose.UpContext(context.Background(), db, "."); err != nil {
		t.Fatalf("goose re-up 00006: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM jobs LIMIT 1"); err != nil {
		t.Fatalf("jobs table should exist again after re-up: %v", err)
	}
}
