//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// #118: the scoped reading repositories must compare user_id and
// library_id with direct equality. A row owned by (alice, lib-a) must be
// invisible to a lookup scoped to (alice, lib-b), and a lookup scoped to
// the correct library must still return it.
func TestReadingRepositories_LibraryScopeIsStrict(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'T')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('ed-1', 'work-1', 'en', '')")
	mustExecPool(t, pool, "INSERT INTO libraries (id, name, description, allow_reader_uploads, created_at, updated_at) VALUES ('lib-a', 'A', '', false, now(), now()) ON CONFLICT DO NOTHING")
	mustExecPool(t, pool, "INSERT INTO libraries (id, name, description, allow_reader_uploads, created_at, updated_at) VALUES ('lib-b', 'B', '', false, now(), now()) ON CONFLICT DO NOTHING")

	now := time.Now().UTC().Truncate(time.Microsecond)

	bm := postgres.NewBookmarkRepository(pool)
	if err := bm.SaveForUser(ctx, "alice", "lib-a", domain.NewBookmark("bm-1", "ed-1", "loc", "l", now)); err != nil {
		t.Fatalf("bookmark SaveForUser: %v", err)
	}
	if list, err := bm.FindByEditionAndUser(ctx, "alice", "lib-b", "ed-1"); err != nil || len(list) != 0 {
		t.Fatalf("bookmark cross-library read: err=%v rows=%d, want 0", err, len(list))
	}
	if list, err := bm.FindByEditionAndUser(ctx, "alice", "lib-a", "ed-1"); err != nil || len(list) != 1 {
		t.Fatalf("bookmark same-library read: err=%v rows=%d, want 1", err, len(list))
	}

	hl := postgres.NewHighlightRepository(pool)
	if err := hl.SaveForUser(ctx, "alice", "lib-a", domain.NewHighlight("hl-1", "ed-1", "s", "e", "n", "", now)); err != nil {
		t.Fatalf("highlight SaveForUser: %v", err)
	}
	if list, err := hl.FindByEditionAndUser(ctx, "alice", "lib-b", "ed-1"); err != nil || len(list) != 0 {
		t.Fatalf("highlight cross-library read: err=%v rows=%d, want 0", err, len(list))
	}
	if list, err := hl.FindByEditionAndUser(ctx, "alice", "lib-a", "ed-1"); err != nil || len(list) != 1 {
		t.Fatalf("highlight same-library read: err=%v rows=%d, want 1", err, len(list))
	}

	pct, _ := domain.NewPercentage(0.5)
	rp := postgres.NewReadingProgressRepository(pool)
	if err := rp.SaveForUser(ctx, "alice", "lib-a", domain.RehydrateReadingProgress("rp-1", "work-1", pct, 1, nil, "", now)); err != nil {
		t.Fatalf("progress SaveForUser: %v", err)
	}
	if _, err := rp.FindByWorkAndUser(ctx, "alice", "lib-b", "work-1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("progress cross-library read: category=%v, want NotFound", domain.CategoryOf(err))
	}
	if _, err := rp.FindByWorkAndUser(ctx, "alice", "lib-a", "work-1"); err != nil {
		t.Fatalf("progress same-library read: %v", err)
	}

	exp := postgres.NewReadingExportRepository(pool)
	if rows, err := exp.ListProgress(ctx, "alice", "lib-b", ""); err != nil || len(rows) != 0 {
		t.Fatalf("export cross-library progress: err=%v rows=%d, want 0", err, len(rows))
	}
	if rows, err := exp.ListProgress(ctx, "alice", "lib-a", ""); err != nil || len(rows) != 1 {
		t.Fatalf("export same-library progress: err=%v rows=%d, want 1", err, len(rows))
	}
	if rows, err := exp.ListMarks(ctx, "alice", "lib-b", ""); err != nil || len(rows) != 0 {
		t.Fatalf("export cross-library marks: err=%v rows=%d, want 0", err, len(rows))
	}
	if rows, err := exp.ListMarks(ctx, "alice", "lib-a", ""); err != nil || len(rows) != 2 {
		t.Fatalf("export same-library marks: err=%v rows=%d, want 2", err, len(rows))
	}
}
