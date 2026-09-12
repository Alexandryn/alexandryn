//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestEditionRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	repo := postgres.NewEditionRepository(pool)

	lang, err := domain.NewLanguage("en")
	if err != nil {
		t.Fatalf("NewLanguage: %v", err)
	}
	isbn := "9780306406157"
	year := 1985
	e, err := domain.NewEdition(
		"edition-1", "work-1", lang, &isbn, "Some Publisher", &year,
		[]domain.ExternalReference{{Source: "openlibrary", ID: "OL1M"}},
	)
	if err != nil {
		t.Fatalf("NewEdition: %v", err)
	}

	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.WorkID() != "work-1" {
		t.Fatalf("WorkID() = %v, want work-1", got.WorkID())
	}
	if got.Language().String() != "en" {
		t.Fatalf("Language() = %v, want en", got.Language())
	}
	if got.ISBN() == nil || *got.ISBN() != isbn {
		t.Fatalf("ISBN() = %v, want %v", got.ISBN(), isbn)
	}
	if got.Publisher() != "Some Publisher" {
		t.Fatalf("Publisher() = %q, want %q", got.Publisher(), "Some Publisher")
	}
	if got.PublicationYear() == nil || *got.PublicationYear() != year {
		t.Fatalf("PublicationYear() = %v, want %v", got.PublicationYear(), year)
	}
	if len(got.ExternalReferences()) != 1 || got.ExternalReferences()[0].ID != "OL1M" {
		t.Fatalf("ExternalReferences() = %v, want [{openlibrary OL1M}]", got.ExternalReferences())
	}
}

func TestEditionRepository_SaveAndFindByID_OptionalFieldsNil(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	repo := postgres.NewEditionRepository(pool)

	lang, err := domain.NewLanguage("en")
	if err != nil {
		t.Fatalf("NewLanguage: %v", err)
	}
	e, err := domain.NewEdition("edition-1", "work-1", lang, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition: %v", err)
	}

	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.ISBN() != nil {
		t.Fatalf("ISBN() = %v, want nil", got.ISBN())
	}
	if got.Publisher() != "" {
		t.Fatalf("Publisher() = %q, want empty", got.Publisher())
	}
	if got.PublicationYear() != nil {
		t.Fatalf("PublicationYear() = %v, want nil", got.PublicationYear())
	}
	if len(got.ExternalReferences()) != 0 {
		t.Fatalf("ExternalReferences() = %v, want empty", got.ExternalReferences())
	}
}

func TestEditionRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewEditionRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestEditionRepository_FindByWork(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-2', 'Other')")
	repo := postgres.NewEditionRepository(pool)

	lang, err := domain.NewLanguage("en")
	if err != nil {
		t.Fatalf("NewLanguage: %v", err)
	}
	e1, err := domain.NewEdition("edition-1", "work-1", lang, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition e1: %v", err)
	}
	e2, err := domain.NewEdition("edition-2", "work-1", lang, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition e2: %v", err)
	}
	e3, err := domain.NewEdition("edition-3", "work-2", lang, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition e3: %v", err)
	}
	for _, e := range []*domain.Edition{e1, e2, e3} {
		if err := repo.Save(ctx, e); err != nil {
			t.Fatalf("Save %v: %v", e.ID(), err)
		}
	}

	got, err := repo.FindByWork(ctx, "work-1")
	if err != nil {
		t.Fatalf("FindByWork: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindByWork = %v, want 2 editions", got)
	}
}

// An Edition cannot exist without a real
// parent Work — proven here against the repository's own Save call, not
// just the schema's FK directly (schema_integration_test.go already
// covers the raw constraint).
func TestEditionRepository_Save_RejectsMissingWork(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewEditionRepository(pool)

	lang, err := domain.NewLanguage("en")
	if err != nil {
		t.Fatalf("NewLanguage: %v", err)
	}
	e, err := domain.NewEdition("edition-1", "nonexistent-work", lang, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition: %v", err)
	}

	err = repo.Save(ctx, e)
	if domain.CategoryOf(err) == "" {
		t.Fatal("Save with a nonexistent work_id succeeded, want an error")
	}
}

// Reuses R4's own SQL-injection proof mechanism (queryTracer,
// sql_injection_tracer_integration_test.go) against EditionRepository's
// own INSERT.
func TestEditionRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	repo := postgres.NewEditionRepository(pool)

	lang, err := domain.NewLanguage("en")
	if err != nil {
		t.Fatalf("NewLanguage: %v", err)
	}
	hostilePublisher := `O'Brien'; DROP TABLE editions; --`

	e, err := domain.NewEdition("edition-injection", "work-1", lang, nil, hostilePublisher, nil, nil)
	if err != nil {
		t.Fatalf("NewEdition: %v", err)
	}
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save with hostile publisher: %v", err)
	}

	got, err := repo.FindByID(ctx, "edition-injection")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Publisher() != hostilePublisher {
		t.Fatalf("Publisher() did not round-trip byte-for-byte: got %q, want %q", got.Publisher(), hostilePublisher)
	}

	var editionsTableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'editions')",
	).Scan(&editionsTableExists); err != nil {
		t.Fatalf("checking editions table existence: %v", err)
	}
	if !editionsTableExists {
		t.Fatal("editions table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO editions")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO editions query — tracer wiring is broken")
	}
	for _, q := range insertQueries {
		if !strings.Contains(q.SQL, "$1") {
			t.Fatalf("traced SQL has no placeholder: %q", q.SQL)
		}
		if strings.Contains(q.SQL, hostilePublisher) {
			t.Fatalf("hostile value interpolated directly into SQL text: %q", q.SQL)
		}
		found := false
		for _, arg := range q.Args {
			if s, ok := arg.(string); ok && s == hostilePublisher {
				found = true
			}
		}
		if !found {
			t.Fatalf("hostile value not present in the query's separate argument list: %v", q.Args)
		}
	}
}
