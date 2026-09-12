//go:build integration

package postgres_test

import (
	"context"
	"testing"
)

// Migration 00012 adds the `system_events` table with
// TEXT foreign keys, JSONB payload, created_at, purge_at, and required indexes.
func TestSchema_SystemEventsTableExistsAfterMigration(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	// 1. Insert a job, user, and library for foreign key resolution
	mustExecPool(t, pool, `INSERT INTO jobs
		(id, kind, payload, status, max_attempts, available_at, created_at, updated_at)
		VALUES ('job-ev-1', 'import', '{}'::jsonb, 'completed', 3, now(), now(), now())`)

	mustExecPool(t, pool, `INSERT INTO libraries (id, name)
		VALUES ('lib-ev-1', 'Event Library')
		ON CONFLICT (id) DO NOTHING`)

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role)
		VALUES ('user-ev-1', 'eventuser', 'event@example.com', 'admin')
		ON CONFLICT (id) DO NOTHING`)

	// 2. Insert into system_events with all fields populated
	mustExecPool(t, pool, `INSERT INTO system_events
		(event_kind, job_id, library_id, user_id, payload, created_at, purge_at)
		VALUES ('job.completed', 'job-ev-1', 'lib-ev-1', 'user-ev-1',
		        '{"items": 5}'::jsonb, now(), now() + interval '30 days')`)

	// 3. Insert host-level event (nullable library_id and user_id)
	mustExecPool(t, pool, `INSERT INTO system_events
		(event_kind, job_id, library_id, user_id, payload, created_at, purge_at)
		VALUES ('host.startup', NULL, NULL, NULL,
		        '{"version": "1.0"}'::jsonb, now(), now() + interval '30 days')`)

	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM system_events`).Scan(&count)
	if err != nil {
		t.Fatalf("COUNT system_events: %v", err)
	}
	if count != 2 {
		t.Fatalf("system_events count = %d, want 2", count)
	}

	// 4. Verify ON DELETE behavior
	// Delete job -> job_id should become NULL
	mustExecPool(t, pool, `DELETE FROM jobs WHERE id = 'job-ev-1'`)
	var jobID *string
	err = pool.QueryRow(ctx, `SELECT job_id FROM system_events WHERE event_kind = 'job.completed'`).Scan(&jobID)
	if err != nil {
		t.Fatalf("scan job_id: %v", err)
	}
	if jobID != nil {
		t.Fatalf("job_id after job delete = %v, want nil (ON DELETE SET NULL)", *jobID)
	}
}
