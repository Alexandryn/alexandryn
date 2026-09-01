//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestHighlightRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewHighlightRepository(pool)

	h := domain.NewHighlight("highlight-1", "edition-1", "loc-10", "loc-20", "Important", "insight", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, h); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "highlight-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", got.EditionID())
	}
	if got.StartPosition() != "loc-10" || got.EndPosition() != "loc-20" {
		t.Fatalf("positions = %q, %q, want loc-10, loc-20", got.StartPosition(), got.EndPosition())
	}
	if got.Note() != "Important" {
		t.Fatalf("Note() = %q, want %q", got.Note(), "Important")
	}
	if got.Category() != "insight" {
		t.Fatalf("Category() = %q, want %q", got.Category(), "insight")
	}
}

func TestHighlightRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewHighlightRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestHighlightRepository_FindByEdition(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-2', 'work-1', 'en', '')")
	repo := postgres.NewHighlightRepository(pool)

	h1 := domain.NewHighlight("highlight-1", "edition-1", "loc-1", "loc-2", "", "", time.Now().UTC().Truncate(time.Microsecond))
	h2 := domain.NewHighlight("highlight-2", "edition-1", "loc-3", "loc-4", "", "", time.Now().UTC().Truncate(time.Microsecond))
	h3 := domain.NewHighlight("highlight-3", "edition-2", "loc-5", "loc-6", "", "", time.Now().UTC().Truncate(time.Microsecond))
	for _, h := range []*domain.Highlight{h1, h2, h3} {
		if err := repo.Save(ctx, h); err != nil {
			t.Fatalf("Save %v: %v", h.ID(), err)
		}
	}

	got, err := repo.FindByEdition(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByEdition: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindByEdition = %v, want 2 highlights", got)
	}
}

func TestHighlightRepository_Save_UpdatesInPlace(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewHighlightRepository(pool)

	h1 := domain.NewHighlight("highlight-1", "edition-1", "loc-1", "loc-2", "Original", "cat-a", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, h1); err != nil {
		t.Fatalf("Save h1: %v", err)
	}

	h2 := domain.NewHighlight("highlight-1", "edition-1", "loc-3", "loc-4", "Updated", "cat-b", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, h2); err != nil {
		t.Fatalf("Save h2: %v", err)
	}

	got, err := repo.FindByID(ctx, "highlight-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Note() != "Updated" || got.Category() != "cat-b" {
		t.Fatalf("got = {%q %q}, want {Updated cat-b}", got.Note(), got.Category())
	}
}

func TestHighlightRepository_Delete(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewHighlightRepository(pool)

	h := domain.NewHighlight("highlight-1", "edition-1", "loc-1", "loc-2", "", "", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, h); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, "highlight-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(ctx, "highlight-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}
}

// Reuses R4's own SQL-injection proof mechanism against
// HighlightRepository's own INSERT.
func TestHighlightRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewHighlightRepository(pool)

	hostileNote := `O'Brien'; DROP TABLE highlights; --`
	h := domain.NewHighlight("highlight-injection", "edition-1", "loc-1", "loc-2", hostileNote, "", time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, h); err != nil {
		t.Fatalf("Save with hostile note: %v", err)
	}

	got, err := repo.FindByID(ctx, "highlight-injection")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Note() != hostileNote {
		t.Fatalf("Note() did not round-trip byte-for-byte: got %q, want %q", got.Note(), hostileNote)
	}

	var tableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'highlights')",
	).Scan(&tableExists); err != nil {
		t.Fatalf("checking highlights table existence: %v", err)
	}
	if !tableExists {
		t.Fatal("highlights table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO highlights")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO highlights query — tracer wiring is broken")
	}
	for _, q := range insertQueries {
		if !strings.Contains(q.SQL, "$1") {
			t.Fatalf("traced SQL has no placeholder: %q", q.SQL)
		}
		if strings.Contains(q.SQL, hostileNote) {
			t.Fatalf("hostile value interpolated directly into SQL text: %q", q.SQL)
		}
		found := false
		for _, arg := range q.Args {
			if s, ok := arg.(string); ok && s == hostileNote {
				found = true
			}
		}
		if !found {
			t.Fatalf("hostile value not present in the query's separate argument list: %v", q.Args)
		}
	}
}
