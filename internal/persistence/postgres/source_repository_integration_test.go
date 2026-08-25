//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestSourceRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRepository(pool)

	caps := domain.SourceCapabilities{CanList: true, CanSearch: true, CanDownload: false}
	s, err := domain.NewSource("source-1", "My Source", caps, "local-folder")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "source-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Label() != "My Source" {
		t.Fatalf("Label() = %q, want %q", got.Label(), "My Source")
	}
	if got.Capabilities() != caps {
		t.Fatalf("Capabilities() = %+v, want %+v", got.Capabilities(), caps)
	}
	if got.Kind() != "local-folder" {
		t.Fatalf("Kind() = %q, want %q", got.Kind(), "local-folder")
	}
}

func TestSourceRepository_Save_UpsertsByID(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRepository(pool)

	caps := domain.SourceCapabilities{CanList: true}
	s1, err := domain.NewSource("source-1", "Original Label", caps, "kind-a")
	if err != nil {
		t.Fatalf("NewSource s1: %v", err)
	}
	if err := repo.Save(ctx, s1); err != nil {
		t.Fatalf("Save s1: %v", err)
	}

	s2, err := domain.NewSource("source-1", "Updated Label", caps, "kind-b")
	if err != nil {
		t.Fatalf("NewSource s2: %v", err)
	}
	if err := repo.Save(ctx, s2); err != nil {
		t.Fatalf("Save s2: %v", err)
	}

	got, err := repo.FindByID(ctx, "source-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Label() != "Updated Label" {
		t.Fatalf("Label() = %q, want %q", got.Label(), "Updated Label")
	}
	if got.Kind() != "kind-b" {
		t.Fatalf("Kind() = %q, want %q", got.Kind(), "kind-b")
	}
}

func TestSourceRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewSourceRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestSourceRepository_Delete(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRepository(pool)

	caps := domain.SourceCapabilities{CanList: true}
	s, err := domain.NewSource("source-1", "My Source", caps, "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, "source-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.FindByID(ctx, "source-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}
}

// Reuses R4's own SQL-injection proof mechanism (queryTracer,
// sql_injection_tracer_integration_test.go) against SourceRepository's own
// INSERT.
func TestSourceRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRepository(pool)

	hostileLabel := `O'Brien'; DROP TABLE sources; --`
	caps := domain.SourceCapabilities{CanList: true}
	s, err := domain.NewSource("source-injection", hostileLabel, caps, "")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("Save with hostile label: %v", err)
	}

	got, err := repo.FindByID(ctx, "source-injection")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Label() != hostileLabel {
		t.Fatalf("Label() did not round-trip byte-for-byte: got %q, want %q", got.Label(), hostileLabel)
	}

	var sourcesTableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'sources')",
	).Scan(&sourcesTableExists); err != nil {
		t.Fatalf("checking sources table existence: %v", err)
	}
	if !sourcesTableExists {
		t.Fatal("sources table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO sources")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO sources query — tracer wiring is broken")
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
