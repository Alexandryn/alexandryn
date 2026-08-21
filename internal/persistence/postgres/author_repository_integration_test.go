//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestAuthorRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewAuthorRepository(pool)

	a, err := domain.NewAuthor("author-1", "Ursula K. Le Guin", []domain.ExternalReference{{Source: "openlibrary", ID: "OL1394244A"}})
	if err != nil {
		t.Fatalf("NewAuthor: %v", err)
	}

	if err := repo.Save(ctx, a); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "author-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name() != "Ursula K. Le Guin" {
		t.Fatalf("Name() = %q, want %q", got.Name(), "Ursula K. Le Guin")
	}
	if len(got.ExternalReferences()) != 1 || got.ExternalReferences()[0].ID != "OL1394244A" {
		t.Fatalf("ExternalReferences() = %v, want [{openlibrary OL1394244A}]", got.ExternalReferences())
	}
	if got.MergedInto() != nil {
		t.Fatalf("MergedInto() = %v, want nil", got.MergedInto())
	}
}

func TestAuthorRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewAuthorRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestAuthorRepository_Save_ReplacesExternalReferencesOnUpdate(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewAuthorRepository(pool)

	mustExecPool(t, pool, "INSERT INTO authors (id, name) VALUES ('author-target', 'Target')")

	a, err := domain.NewAuthor("author-1", "Name", []domain.ExternalReference{{Source: "openlibrary", ID: "OL1A"}})
	if err != nil {
		t.Fatalf("NewAuthor: %v", err)
	}
	if err := repo.Save(ctx, a); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	target := domain.AuthorID("author-target")
	updated := domain.RehydrateAuthor("author-1", "Name", []domain.ExternalReference{{Source: "viaf", ID: "V1"}}, &target)
	if err := repo.Save(ctx, updated); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "author-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(got.ExternalReferences()) != 1 || got.ExternalReferences()[0].Source != "viaf" {
		t.Fatalf("ExternalReferences() = %v, want exactly [{viaf V1}], not accumulated with openlibrary", got.ExternalReferences())
	}
	if got.MergedInto() == nil || *got.MergedInto() != "author-target" {
		t.Fatalf("MergedInto() = %v, want author-target", got.MergedInto())
	}
}

// Reuses R4's own SQL-injection proof mechanism (queryTracer,
// sql_injection_helper_test.go) against AuthorRepository's own INSERT.
func TestAuthorRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	repo := postgres.NewAuthorRepository(pool)

	hostileName := `O'Brien'; DROP TABLE authors; --`

	a, err := domain.NewAuthor("author-injection", hostileName, nil)
	if err != nil {
		t.Fatalf("NewAuthor: %v", err)
	}
	if err := repo.Save(ctx, a); err != nil {
		t.Fatalf("Save with hostile name: %v", err)
	}

	got, err := repo.FindByID(ctx, "author-injection")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name() != hostileName {
		t.Fatalf("Name() did not round-trip byte-for-byte: got %q, want %q", got.Name(), hostileName)
	}

	var authorsTableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'authors')",
	).Scan(&authorsTableExists); err != nil {
		t.Fatalf("checking authors table existence: %v", err)
	}
	if !authorsTableExists {
		t.Fatal("authors table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO authors")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO authors query — tracer wiring is broken")
	}
	for _, q := range insertQueries {
		if !strings.Contains(q.SQL, "$1") {
			t.Fatalf("traced SQL has no placeholder: %q", q.SQL)
		}
		if strings.Contains(q.SQL, hostileName) {
			t.Fatalf("hostile value interpolated directly into SQL text: %q", q.SQL)
		}
		found := false
		for _, arg := range q.Args {
			if s, ok := arg.(string); ok && s == hostileName {
				found = true
			}
		}
		if !found {
			t.Fatalf("hostile value not present in the query's separate argument list: %v", q.Args)
		}
	}
}
