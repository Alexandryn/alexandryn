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

func TestWorkRepository_QueryLibrary_FiltersAndPagination(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	// Seed 3 works:
	// w1: owned (has edition + library entry)
	// w2: wanted (in collection, no library entry)
	// w3: both owned and in collection
	mustExecPool(t, pool, `
		INSERT INTO works (id, title) VALUES ('w1', 'Alpha Book'), ('w2', 'Beta Book'), ('w3', 'Gamma Book');
		INSERT INTO authors (id, name) VALUES ('a1', 'Arthur Conan Doyle'), ('a2', 'Jane Austen');
		INSERT INTO work_authors (work_id, author_id) VALUES ('w1', 'a1'), ('w2', 'a2'), ('w3', 'a1');
		INSERT INTO editions (id, work_id, language) VALUES ('e1', 'w1', 'en'), ('e3', 'w3', 'en');
		INSERT INTO library_entries (id, edition_id, added_at) VALUES
			('le1', 'e1', '2026-01-01T10:00:00Z'),
			('le3', 'e3', '2026-01-03T10:00:00Z');
		INSERT INTO collections (id, name) VALUES ('c1', 'Sci-Fi'), ('c2', 'Favorites');
		INSERT INTO collection_members (collection_id, work_id, added_at) VALUES
			('c1', 'w2', '2026-01-02T10:00:00Z'),
			('c2', 'w3', '2026-01-04T10:00:00Z');
	`)

	// 1. Filter: owned
	page, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Filter: domain.FilterOwned, Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary(FilterOwned): %v", err)
	}
	if len(page.Works) != 2 {
		t.Fatalf("owned count = %d, want 2", len(page.Works))
	}
	// Sorted by added_at desc: w3 (2026-01-03), w1 (2026-01-01)
	if page.Works[0].ID != "w3" || page.Works[1].ID != "w1" {
		t.Errorf("owned order = [%s, %s], want [w3, w1]", page.Works[0].ID, page.Works[1].ID)
	}

	// 2. Filter: wanted
	page, err = repo.QueryLibrary(ctx, domain.LibraryQuery{Filter: domain.FilterWanted, Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary(FilterWanted): %v", err)
	}
	if len(page.Works) != 1 || page.Works[0].ID != "w2" {
		t.Fatalf("wanted results = %v, want [w2]", page.Works)
	}
	if page.Works[0].IsOwned {
		t.Errorf("w2 isOwned = true, want false")
	}

	// 3. Filter: all (default)
	page, err = repo.QueryLibrary(ctx, domain.LibraryQuery{Filter: domain.FilterAll, Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary(FilterAll): %v", err)
	}
	if len(page.Works) != 3 {
		t.Fatalf("all count = %d, want 3", len(page.Works))
	}

	// 4. Sort: title (ascending)
	page, err = repo.QueryLibrary(ctx, domain.LibraryQuery{Filter: domain.FilterAll, Sort: domain.SortTitle, Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary(SortTitle): %v", err)
	}
	if len(page.Works) != 3 || page.Works[0].ID != "w1" || page.Works[1].ID != "w2" || page.Works[2].ID != "w3" {
		t.Errorf("title sort order = [%s, %s, %s], want [w1, w2, w3]", page.Works[0].ID, page.Works[1].ID, page.Works[2].ID)
	}

	// 5. Search: ?q=Doyle
	page, err = repo.QueryLibrary(ctx, domain.LibraryQuery{Q: "Doyle", Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary(Q=Doyle): %v", err)
	}
	if len(page.Works) != 2 {
		t.Errorf("search count = %d, want 2 (w1, w3)", len(page.Works))
	}
}

// TestWorkRepository_LibraryScoping is the audit 0016 #88 regression: a
// member of library A must never see library B's holdings via
// GET /api/v1/library or GET /api/v1/works/{id}.
func TestWorkRepository_LibraryScoping(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	const libA = "00000000-0000-0000-0000-000000000001" // default
	const libB = "00000000-0000-0000-0000-0000000000b0"

	mustExecPool(t, pool, `
		INSERT INTO libraries (id, name, allow_reader_uploads, created_at, updated_at)
			VALUES ('`+libB+`', 'Library B', FALSE, now(), now());
		INSERT INTO works (id, title) VALUES ('wa', 'A Only Book'), ('wb', 'B Only Book');
		INSERT INTO editions (id, work_id, language) VALUES ('ea', 'wa', 'en'), ('eb', 'wb', 'en');
		INSERT INTO library_entries (id, edition_id, library_id, added_at) VALUES
			('lea', 'ea', '`+libA+`', '2026-02-01T10:00:00Z'),
			('leb', 'eb', '`+libB+`', '2026-02-02T10:00:00Z');
		INSERT INTO collections (id, name, library_id) VALUES ('cb', 'B Shelf', '`+libB+`');
		INSERT INTO collection_members (collection_id, work_id, added_at) VALUES ('cb', 'wb', '2026-02-03T10:00:00Z');
	`)

	// Library A sees only wa.
	pageA, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Filter: domain.FilterAll, Limit: 10, LibraryID: libA})
	if err != nil {
		t.Fatalf("QueryLibrary(A): %v", err)
	}
	if len(pageA.Works) != 1 || pageA.Works[0].ID != "wa" {
		t.Fatalf("library A page = %+v, want [wa] only (no cross-library disclosure)", ids(pageA.Works))
	}

	// Library B sees only wb.
	pageB, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Filter: domain.FilterAll, Limit: 10, LibraryID: libB})
	if err != nil {
		t.Fatalf("QueryLibrary(B): %v", err)
	}
	if len(pageB.Works) != 1 || pageB.Works[0].ID != "wb" {
		t.Fatalf("library B page = %+v, want [wb] only", ids(pageB.Works))
	}

	// FindWorkDetail: A cannot read wb's owned-edition detail.
	if _, err := repo.FindWorkDetail(ctx, "wb", libA); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("FindWorkDetail(wb, libA) err = %v, want NotFound", err)
	}
	// B can, and sees the edition + collection.
	det, err := repo.FindWorkDetail(ctx, "wb", libB)
	if err != nil {
		t.Fatalf("FindWorkDetail(wb, libB): %v", err)
	}
	if len(det.OwnedEditions) != 1 || len(det.Collections) != 1 {
		t.Fatalf("wb detail in B = %d editions, %d collections; want 1 and 1", len(det.OwnedEditions), len(det.Collections))
	}
	// A reading wa's detail must not surface B's collection membership.
	detA, err := repo.FindWorkDetail(ctx, "wa", libA)
	if err != nil {
		t.Fatalf("FindWorkDetail(wa, libA): %v", err)
	}
	if len(detA.Collections) != 0 {
		t.Fatalf("wa detail in A leaked %d collections", len(detA.Collections))
	}
}

func ids(ws []*domain.WorkSummary) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = string(w.ID)
	}
	return out
}

func TestWorkRepository_QueryLibrary_CursorStability(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	// Seed 4 works sorted by added_at desc:
	// w4: 2026-01-04, w3: 2026-01-03, w2: 2026-01-02, w1: 2026-01-01
	mustExecPool(t, pool, `
		INSERT INTO works (id, title) VALUES ('w1', 'Book 1'), ('w2', 'Book 2'), ('w3', 'Book 3'), ('w4', 'Book 4');
		INSERT INTO editions (id, work_id, language) VALUES ('e1', 'w1', 'en'), ('e2', 'w2', 'en'), ('e3', 'w3', 'en'), ('e4', 'w4', 'en');
		INSERT INTO library_entries (id, edition_id, added_at) VALUES
			('le1', 'e1', '2026-01-01T10:00:00Z'),
			('le2', 'e2', '2026-01-02T10:00:00Z'),
			('le3', 'e3', '2026-01-03T10:00:00Z'),
			('le4', 'e4', '2026-01-04T10:00:00Z');
	`)

	// Scenario (a): fetch page 1 (limit 2) -> [w4, w3], insert new work w5 (sorts above cursor at 2026-01-05), fetch page 2
	p1, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Limit: 2})
	if err != nil {
		t.Fatalf("p1: %v", err)
	}
	if len(p1.Works) != 2 || p1.Works[0].ID != "w4" || p1.Works[1].ID != "w3" {
		t.Fatalf("p1 works = %v, want [w4, w3]", p1.Works)
	}
	if p1.NextCursor == "" {
		t.Fatal("expected non-empty nextCursor on p1")
	}

	mustExecPool(t, pool, `
		INSERT INTO works (id, title) VALUES ('w5', 'Book 5');
		INSERT INTO editions (id, work_id, language) VALUES ('e5', 'w5', 'en');
		INSERT INTO library_entries (id, edition_id, added_at) VALUES ('le5', 'e5', '2026-01-05T10:00:00Z');
	`)

	p2, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Cursor: p1.NextCursor, Limit: 2})
	if err != nil {
		t.Fatalf("p2: %v", err)
	}
	if len(p2.Works) != 2 || p2.Works[0].ID != "w2" || p2.Works[1].ID != "w1" {
		t.Fatalf("p2 works = %v, want [w2, w1] (no duplicates or skips)", p2.Works)
	}

	// Scenario (b): delete row below cursor (w1), fetch page 2 from original p1 cursor
	mustExecPool(t, pool, `
		DELETE FROM library_entries WHERE id = 'le1';
		DELETE FROM editions WHERE id = 'e1';
		DELETE FROM works WHERE id = 'w1';
	`)
	p2b, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Cursor: p1.NextCursor, Limit: 2})
	if err != nil {
		t.Fatalf("p2b: %v", err)
	}
	if len(p2b.Works) != 1 || p2b.Works[0].ID != "w2" {
		t.Fatalf("p2b works = %v, want [w2]", p2b.Works)
	}
	if p2b.NextCursor != "" {
		t.Fatalf("p2b nextCursor = %q, want empty (last page)", p2b.NextCursor)
	}

	// Scenario (c): Repeated under sort=title (alphabetical A-Z, total ordering with id tiebreaker)
	// Clean reset & seed 4 works with known titles:
	// "Atlas", "Brave New World", "Catch-22", "Dracula"
	mustExecPool(t, pool, `
		DELETE FROM library_entries;
		DELETE FROM editions;
		DELETE FROM works;
		INSERT INTO works (id, title) VALUES ('w-a', 'Atlas'), ('w-b', 'Brave New World'), ('w-c', 'Catch-22'), ('w-d', 'Dracula');
		INSERT INTO editions (id, work_id, language) VALUES ('ea', 'w-a', 'en'), ('eb', 'w-b', 'en'), ('ec', 'w-c', 'en'), ('ed', 'w-d', 'en');
		INSERT INTO library_entries (id, edition_id, added_at) VALUES
			('lea', 'ea', '2026-01-01T10:00:00Z'),
			('leb', 'eb', '2026-01-01T10:00:00Z'),
			('lec', 'ec', '2026-01-01T10:00:00Z'),
			('led', 'ed', '2026-01-01T10:00:00Z');
	`)

	// Fetch page 1 (limit 2) under sort=title -> [w-a "Atlas", w-b "Brave New World"]
	pTitle1, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Sort: domain.SortTitle, Limit: 2})
	if err != nil {
		t.Fatalf("pTitle1: %v", err)
	}
	if len(pTitle1.Works) != 2 || pTitle1.Works[0].ID != "w-a" || pTitle1.Works[1].ID != "w-b" {
		t.Fatalf("pTitle1 works = %v, want [w-a, w-b]", pTitle1.Works)
	}
	if pTitle1.NextCursor == "" {
		t.Fatal("expected non-empty nextCursor on pTitle1")
	}

	// Insert "Anna Karenina" (sorts BEFORE current cursor position alphabetically) and "Frankenstein" (sorts AFTER)
	mustExecPool(t, pool, `
		INSERT INTO works (id, title) VALUES ('w-anna', 'Anna Karenina'), ('w-f', 'Frankenstein');
		INSERT INTO editions (id, work_id, language) VALUES ('e-anna', 'w-anna', 'en'), ('ef', 'w-f', 'en');
		INSERT INTO library_entries (id, edition_id, added_at) VALUES
			('le-anna', 'e-anna', '2026-01-01T10:00:00Z'),
			('lef', 'ef', '2026-01-01T10:00:00Z');
	`)

	// Fetch page 2 from pTitle1 cursor -> should return [w-c "Catch-22", w-d "Dracula"]
	pTitle2, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Sort: domain.SortTitle, Cursor: pTitle1.NextCursor, Limit: 2})
	if err != nil {
		t.Fatalf("pTitle2: %v", err)
	}
	if len(pTitle2.Works) != 2 || pTitle2.Works[0].ID != "w-c" || pTitle2.Works[1].ID != "w-d" {
		t.Fatalf("pTitle2 works = %v, want [w-c, w-d]", pTitle2.Works)
	}

	// Fetch page 3 from pTitle2 cursor -> should return [w-f "Frankenstein"]
	pTitle3, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Sort: domain.SortTitle, Cursor: pTitle2.NextCursor, Limit: 2})
	if err != nil {
		t.Fatalf("pTitle3: %v", err)
	}
	if len(pTitle3.Works) != 1 || pTitle3.Works[0].ID != "w-f" {
		t.Fatalf("pTitle3 works = %v, want [w-f]", pTitle3.Works)
	}
	if pTitle3.NextCursor != "" {
		t.Fatalf("pTitle3 nextCursor = %q, want empty (last page)", pTitle3.NextCursor)
	}
}


func TestWorkRepository_FindWorkDetail(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	mustExecPool(t, pool, `
		INSERT INTO works (id, title, subtitle, original_language) VALUES ('w-det', 'Dune', 'Chronicles', 'en');
		INSERT INTO authors (id, name) VALUES ('a-fh', 'Frank Herbert');
		INSERT INTO work_authors (work_id, author_id) VALUES ('w-det', 'a-fh');
		INSERT INTO work_subjects (work_id, subject) VALUES ('w-det', 'Science Fiction');
		INSERT INTO editions (id, work_id, language, publisher, publication_year) VALUES ('e-det1', 'w-det', 'en', 'Chilton Books', 1965);
		INSERT INTO library_entries (id, edition_id, added_at) VALUES ('le-det1', 'e-det1', '2026-01-10T12:00:00Z');
		INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('src-1', 'Local', true, true, true, 'local');
		INSERT INTO source_offerings (id, source_id, edition_id, file_reference_id, file_reference_format, observed_at) VALUES
			('so-1', 'src-1', 'e-det1', 'f1', 'epub', '2026-01-10T12:00:00Z'),
			('so-2', 'src-1', 'e-det1', 'f2', 'pdf', '2026-01-10T12:00:00Z');
		INSERT INTO collections (id, name) VALUES ('c-det1', 'Sci-Fi Masterpieces');
		INSERT INTO collection_members (collection_id, work_id, added_at) VALUES ('c-det1', 'w-det', '2026-01-11T12:00:00Z');
	`)

	detail, err := repo.FindWorkDetail(ctx, "w-det", domain.DefaultLibraryID)
	if err != nil {
		t.Fatalf("FindWorkDetail: %v", err)
	}

	if detail.Title != "Dune" || detail.Subtitle != "Chronicles" {
		t.Errorf("title/subtitle = %q/%q, want Dune/Chronicles", detail.Title, detail.Subtitle)
	}
	if len(detail.Authors) != 1 || detail.Authors[0] != "Frank Herbert" {
		t.Errorf("authors = %v, want [Frank Herbert]", detail.Authors)
	}
	if len(detail.Subjects) != 1 || detail.Subjects[0] != "Science Fiction" {
		t.Errorf("subjects = %v, want [Science Fiction]", detail.Subjects)
	}
	if len(detail.OwnedEditions) != 1 {
		t.Fatalf("owned editions count = %d, want 1", len(detail.OwnedEditions))
	}
	e := detail.OwnedEditions[0]
	if e.Publisher != "Chilton Books" || *e.PublicationYear != 1965 {
		t.Errorf("edition pub = %s/%d", e.Publisher, *e.PublicationYear)
	}
	if len(e.Formats) != 2 || e.Formats[0] != "epub" || e.Formats[1] != "pdf" {
		t.Errorf("edition formats = %v, want [epub, pdf]", e.Formats)
	}
	if len(detail.Collections) != 1 || detail.Collections[0].Name != "Sci-Fi Masterpieces" {
		t.Errorf("collections = %v, want [Sci-Fi Masterpieces]", detail.Collections)
	}

	// 404 test
	_, err = repo.FindWorkDetail(ctx, "nonexistent-work", domain.DefaultLibraryID)
	if domain.CategoryOf(err) != domain.NotFound {
		t.Errorf("FindWorkDetail(nonexistent) error category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestWorkRepository_QueryLibrary_SingleQueryClaim(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	repo := postgres.NewWorkRepository(pool)

	mustExecPool(t, pool, `
		INSERT INTO works (id, title) VALUES ('w1', 'Book 1'), ('w2', 'Book 2'), ('w3', 'Book 3');
		INSERT INTO editions (id, work_id, language) VALUES ('e1', 'w1', 'en'), ('e2', 'w2', 'en');
		INSERT INTO library_entries (id, edition_id, added_at) VALUES
			('le1', 'e1', '2026-01-01T10:00:00Z'),
			('le2', 'e2', '2026-01-02T10:00:00Z');
	`)

	tracer.mu.Lock()
	tracer.queries = nil
	tracer.mu.Unlock()

	page, err := repo.QueryLibrary(ctx, domain.LibraryQuery{Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary: %v", err)
	}
	if len(page.Works) != 2 {
		t.Fatalf("page.Works count = %d, want 2", len(page.Works))
	}

	tracer.mu.Lock()
	queryCount := len(tracer.queries)
	tracedQueries := make([]tracedQuery, len(tracer.queries))
	copy(tracedQueries, tracer.queries)
	tracer.mu.Unlock()

	if queryCount != 1 {
		t.Fatalf("backend-library-api.md FR-9 single query claim failed: executed %d queries, want exactly 1. Queries: %+v", queryCount, tracedQueries)
	}
}

