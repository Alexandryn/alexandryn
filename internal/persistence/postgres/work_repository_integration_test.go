//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestWorkRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	subject, err := domain.NewSubject("Fantasy")
	if err != nil {
		t.Fatalf("NewSubject: %v", err)
	}
	lang, err := domain.NewLanguage("en-US")
	if err != nil {
		t.Fatalf("NewLanguage: %v", err)
	}
	w, err := domain.NewWork(
		"work-1", "The Title", "A Subtitle",
		[]domain.AuthorID{"author-1", "author-2"},
		[]domain.Subject{subject},
		&lang,
		[]domain.ExternalReference{{Source: "openlibrary", ID: "OL123W"}},
	)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}

	if err := repo.Save(ctx, w); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Title() != "The Title" || got.Subtitle() != "A Subtitle" {
		t.Fatalf("Title/Subtitle = %q/%q, want %q/%q", got.Title(), got.Subtitle(), "The Title", "A Subtitle")
	}
	if !containsAuthorID(got.Authors(), "author-1") || !containsAuthorID(got.Authors(), "author-2") || len(got.Authors()) != 2 {
		t.Fatalf("Authors() = %v, want exactly [author-1 author-2] (any order)", got.Authors())
	}
	if len(got.Subjects()) != 1 || got.Subjects()[0].String() != "Fantasy" {
		t.Fatalf("Subjects() = %v, want [Fantasy]", got.Subjects())
	}
	if got.OriginalLanguage() == nil || got.OriginalLanguage().String() != "en-US" {
		t.Fatalf("OriginalLanguage() = %v, want en-US", got.OriginalLanguage())
	}
	if len(got.ExternalReferences()) != 1 || got.ExternalReferences()[0].ID != "OL123W" {
		t.Fatalf("ExternalReferences() = %v, want [{openlibrary OL123W}]", got.ExternalReferences())
	}
	if got.MergedInto() != nil {
		t.Fatalf("MergedInto() = %v, want nil", got.MergedInto())
	}
	if len(got.Contains()) != 0 {
		t.Fatalf("Contains() = %v, want empty", got.Contains())
	}
}

func containsAuthorID(ids []domain.AuthorID, want domain.AuthorID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestWorkRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewWorkRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

// Save must replace, not append, every child-table row for a Work's id —
// otherwise a merge or containment update recorded via RehydrateWork and
// re-Saved would accumulate stale rows alongside the new ones.
func TestWorkRepository_Save_ReplacesChildRowsOnUpdate(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-target', 'Target')")

	w, err := domain.NewWork("work-1", "Title", "", []domain.AuthorID{"author-1"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}
	if err := repo.Save(ctx, w); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	target := domain.WorkID("work-target")
	updated := domain.RehydrateWork("work-1", "Title", "", []domain.AuthorID{"author-2"}, nil, nil, nil, &target, []domain.WorkID{"work-target"})
	if err := repo.Save(ctx, updated); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(got.Authors()) != 1 || got.Authors()[0] != "author-2" {
		t.Fatalf("Authors() = %v, want exactly [author-2], not accumulated with author-1", got.Authors())
	}
	if got.MergedInto() == nil || *got.MergedInto() != "work-target" {
		t.Fatalf("MergedInto() = %v, want work-target", got.MergedInto())
	}
}

func TestWorkRepository_FindMergedInto(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	canonical, err := domain.NewWork("work-canonical", "Canonical", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}
	if err := repo.Save(ctx, canonical); err != nil {
		t.Fatalf("Save canonical: %v", err)
	}

	target := domain.WorkID("work-canonical")
	dupe := domain.RehydrateWork("work-dupe", "Dupe", "", nil, nil, nil, nil, &target, nil)
	if err := repo.Save(ctx, dupe); err != nil {
		t.Fatalf("Save dupe: %v", err)
	}
	other, err := domain.NewWork("work-other", "Other", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork other: %v", err)
	}
	if err := repo.Save(ctx, other); err != nil {
		t.Fatalf("Save other: %v", err)
	}

	got, err := repo.FindMergedInto(ctx, "work-canonical")
	if err != nil {
		t.Fatalf("FindMergedInto: %v", err)
	}
	if len(got) != 1 || got[0].ID() != "work-dupe" {
		t.Fatalf("FindMergedInto = %v, want exactly [work-dupe]", got)
	}
}

// backend-persistence.md FR-3's adversarial proof, built once (this test)
// and reused by every later repository (T24's own R5-R8): a hostile value
// in a free-form text field round-trips byte-for-byte, the works table
// still exists afterward, no syntax error occurs, and — the real proof, not
// just a black-box coincidence — the literal SQL text pgx sent carries a
// placeholder at the hostile field's position, with the hostile value
// present only in the separate argument list.
func TestWorkRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	hostileTitle := `O'Brien'; DROP TABLE works; --`
	hostileSubtitle := `x' OR '1'='1`
	hostileSubject, err := domain.NewSubject(`/*comment*/ UNION SELECT * FROM works --`)
	if err != nil {
		t.Fatalf("NewSubject: %v", err)
	}

	w, err := domain.NewWork(
		"work-injection", hostileTitle, hostileSubtitle,
		nil, []domain.Subject{hostileSubject}, nil,
		[]domain.ExternalReference{{Source: "openlibrary", ID: hostileTitle}},
	)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}

	if err := repo.Save(ctx, w); err != nil {
		t.Fatalf("Save with hostile values: %v", err)
	}

	got, err := repo.FindByID(ctx, "work-injection")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Title() != hostileTitle {
		t.Fatalf("Title() did not round-trip byte-for-byte: got %q, want %q", got.Title(), hostileTitle)
	}
	if got.Subtitle() != hostileSubtitle {
		t.Fatalf("Subtitle() did not round-trip byte-for-byte: got %q, want %q", got.Subtitle(), hostileSubtitle)
	}
	if len(got.Subjects()) != 1 || got.Subjects()[0].String() != hostileSubject.String() {
		t.Fatalf("Subjects() did not round-trip: got %v", got.Subjects())
	}

	var worksTableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'works')",
	).Scan(&worksTableExists); err != nil {
		t.Fatalf("checking works table existence: %v", err)
	}
	if !worksTableExists {
		t.Fatal("works table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO works")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO works query — tracer wiring is broken")
	}
	for _, q := range insertQueries {
		if !strings.Contains(q.SQL, "$1") {
			t.Fatalf("traced SQL has no placeholder: %q", q.SQL)
		}
		if strings.Contains(q.SQL, hostileTitle) {
			t.Fatalf("hostile value interpolated directly into SQL text: %q", q.SQL)
		}
		found := false
		for _, arg := range q.Args {
			if s, ok := arg.(string); ok && s == hostileTitle {
				found = true
			}
		}
		if !found {
			t.Fatalf("hostile value not present in the query's separate argument list: %v", q.Args)
		}
	}
}
