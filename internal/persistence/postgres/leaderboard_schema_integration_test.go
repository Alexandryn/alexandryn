//go:build integration

package postgres_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// FinishedWorksHandler and LibraryLeaderboardHandler both
// filter reading_progress by (library_id, percentage >= 100). Migration
// 00013 adds a partial index so that hot path does not sequential-scan.
func TestSchema_LeaderboardFinishedIndexExists(t *testing.T) {
	pool := schemaTestPool(t)

	var indexdef string
	err := pool.QueryRow(context.Background(),
		`SELECT indexdef FROM pg_indexes
		 WHERE tablename = 'reading_progress'
		   AND indexname = 'reading_progress_library_finished_idx'`,
	).Scan(&indexdef)
	if err != nil {
		t.Fatalf("index reading_progress_library_finished_idx not found: %v", err)
	}
	if !strings.Contains(indexdef, "WHERE") || !strings.Contains(indexdef, "percentage >= ") {
		t.Fatalf("index must be partial on percentage >= 100, got: %q", indexdef)
	}
	if !strings.Contains(indexdef, "library_id") {
		t.Fatalf("index must lead with library_id, got: %q", indexdef)
	}
}

// Verifies that the down migration is
// reversible and re-appliable.
func TestSchema_LeaderboardIndexMigrationIsReversible(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)
	if err := postgres.Migrate(context.Background(), os.Getenv("TEST_DATABASE_URL")); err != nil {
		t.Fatalf("Migrate up: %v", err)
	}

	goose.SetBaseFS(os.DirFS("migrations"))
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("SetDialect: %v", err)
	}
	if err := goose.DownToContext(context.Background(), db, ".", 12); err != nil {
		t.Fatalf("goose down to 00012: %v", err)
	}
	var exists bool
	if err := db.QueryRowContext(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'reading_progress_library_finished_idx')`,
	).Scan(&exists); err != nil {
		t.Fatalf("checking index after down: %v", err)
	}
	if exists {
		t.Fatal("index still present after down migration 00013")
	}
	if err := goose.UpContext(context.Background(), db, "."); err != nil {
		t.Fatalf("goose re-up 00013: %v", err)
	}
}
