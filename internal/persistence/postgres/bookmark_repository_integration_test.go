//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestBookmarkRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewBookmarkRepository(pool)

	b := domain.NewBookmark("bookmark-1", "edition-1", "loc-10", "Chapter start")
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "bookmark-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", got.EditionID())
	}
	if got.Position() != "loc-10" {
		t.Fatalf("Position() = %q, want loc-10", got.Position())
	}
	if got.Label() != "Chapter start" {
		t.Fatalf("Label() = %q, want %q", got.Label(), "Chapter start")
	}
}

func TestBookmarkRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewBookmarkRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestBookmarkRepository_FindByEdition(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-2', 'work-1', 'en', '')")
	repo := postgres.NewBookmarkRepository(pool)

	b1 := domain.NewBookmark("bookmark-1", "edition-1", "loc-1", "")
	b2 := domain.NewBookmark("bookmark-2", "edition-1", "loc-2", "")
	b3 := domain.NewBookmark("bookmark-3", "edition-2", "loc-3", "")
	for _, b := range []*domain.Bookmark{b1, b2, b3} {
		if err := repo.Save(ctx, b); err != nil {
			t.Fatalf("Save %v: %v", b.ID(), err)
		}
	}

	got, err := repo.FindByEdition(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByEdition: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindByEdition = %v, want 2 bookmarks", got)
	}
}

func TestBookmarkRepository_Save_UpdatesInPlace(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewBookmarkRepository(pool)

	b1 := domain.NewBookmark("bookmark-1", "edition-1", "loc-1", "Original")
	if err := repo.Save(ctx, b1); err != nil {
		t.Fatalf("Save b1: %v", err)
	}

	b2 := domain.NewBookmark("bookmark-1", "edition-1", "loc-2", "Updated")
	if err := repo.Save(ctx, b2); err != nil {
		t.Fatalf("Save b2: %v", err)
	}

	got, err := repo.FindByID(ctx, "bookmark-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Position() != "loc-2" || got.Label() != "Updated" {
		t.Fatalf("got = {%q %q}, want {loc-2 Updated}", got.Position(), got.Label())
	}
}

func TestBookmarkRepository_Delete(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewBookmarkRepository(pool)

	b := domain.NewBookmark("bookmark-1", "edition-1", "loc-1", "")
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, "bookmark-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(ctx, "bookmark-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}
}

// Reuses R4's own SQL-injection proof mechanism against
// BookmarkRepository's own INSERT.
func TestBookmarkRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewBookmarkRepository(pool)

	hostileLabel := `O'Brien'; DROP TABLE bookmarks; --`
	b := domain.NewBookmark("bookmark-injection", "edition-1", "loc-1", hostileLabel)
	if err := repo.Save(ctx, b); err != nil {
		t.Fatalf("Save with hostile label: %v", err)
	}

	got, err := repo.FindByID(ctx, "bookmark-injection")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Label() != hostileLabel {
		t.Fatalf("Label() did not round-trip byte-for-byte: got %q, want %q", got.Label(), hostileLabel)
	}

	var tableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'bookmarks')",
	).Scan(&tableExists); err != nil {
		t.Fatalf("checking bookmarks table existence: %v", err)
	}
	if !tableExists {
		t.Fatal("bookmarks table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO bookmarks")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO bookmarks query — tracer wiring is broken")
	}
	for _, q := range insertQueries {
		if !strings.Contains(q.SQL, "$1") {
			t.Fatalf("traced SQL has no placeholder: %q", q.SQL)
		}
		if strings.Contains(q.SQL, hostileLabel) {
			t.Fatalf("hostile value interpolated directly into SQL text: %q", q.SQL)
		}
		found := false
		for _, arg := range q.Args {
			if s, ok := arg.(string); ok && s == hostileLabel {
				found = true
			}
		}
		if !found {
			t.Fatalf("hostile value not present in the query's separate argument list: %v", q.Args)
		}
	}
}
