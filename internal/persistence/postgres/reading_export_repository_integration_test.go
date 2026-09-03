//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// TestReadingExportRepository_ScopedToUser is the AUDIT-0012-C1
// close-gate integration test for GET /api/v1/reading/export: the bulk
// read returns only the calling user's rows, never the instance's.
func TestReadingExportRepository_ScopedToUser(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'T')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('ed-1', 'work-1', 'en', '')")

	bm := postgres.NewBookmarkRepository(pool)
	hl := postgres.NewHighlightRepository(pool)
	prog := postgres.NewReadingProgressRepository(pool)
	exp := postgres.NewReadingExportRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	pct, _ := domain.NewPercentage(0.4)
	if err := prog.SaveForUser(ctx, "alice", "", domain.RehydrateReadingProgress("rp-a", "work-1", pct, 1, nil, "", now)); err != nil {
		t.Fatalf("alice progress: %v", err)
	}
	pctB, _ := domain.NewPercentage(0.9)
	if err := prog.SaveForUser(ctx, "bob", "", domain.RehydrateReadingProgress("rp-b", "work-1", pctB, 1, nil, "", now)); err != nil {
		t.Fatalf("bob progress: %v", err)
	}
	if err := bm.SaveForUser(ctx, "alice", "", domain.NewBookmark("bm-a", "ed-1", "loc", "alice-secret", now)); err != nil {
		t.Fatalf("alice bookmark: %v", err)
	}
	if err := hl.SaveForUser(ctx, "bob", "", domain.NewHighlight("hl-b", "ed-1", "s", "e", "bob-note", "", now)); err != nil {
		t.Fatalf("bob highlight: %v", err)
	}

	aliceProg, err := exp.ListProgress(ctx, "alice", "", "")
	if err != nil {
		t.Fatalf("ListProgress(alice): %v", err)
	}
	if len(aliceProg) != 1 || aliceProg[0].Epoch != 1 {
		t.Fatalf("ListProgress(alice) = %+v, want exactly alice's one row", aliceProg)
	}

	aliceMarks, err := exp.ListMarks(ctx, "alice", "", "")
	if err != nil {
		t.Fatalf("ListMarks(alice): %v", err)
	}
	for _, m := range aliceMarks {
		if m.Note == "bob-note" {
			t.Fatalf("alice's export leaked bob's highlight note: %+v", aliceMarks)
		}
	}
	if len(aliceMarks) != 1 || aliceMarks[0].Label != "alice-secret" {
		t.Fatalf("ListMarks(alice) = %+v, want exactly alice's one bookmark", aliceMarks)
	}

	bobMarks, err := exp.ListMarks(ctx, "bob", "", "")
	if err != nil {
		t.Fatalf("ListMarks(bob): %v", err)
	}
	if len(bobMarks) != 1 || bobMarks[0].Note != "bob-note" {
		t.Fatalf("ListMarks(bob) = %+v, want exactly bob's one highlight", bobMarks)
	}
}
