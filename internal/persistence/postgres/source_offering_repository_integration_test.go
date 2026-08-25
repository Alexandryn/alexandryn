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

func TestSourceOfferingRepository_SaveAndFindByID_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src', true, true, true, '')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewSourceOfferingRepository(pool)

	ref, err := domain.NewFileReference("ref-1", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference: %v", err)
	}
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	o := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref, observedAt)

	if err := repo.Save(ctx, o); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "offering-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.SourceID() != "source-1" {
		t.Fatalf("SourceID() = %v, want source-1", got.SourceID())
	}
	if got.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", got.EditionID())
	}
	if got.FileReference() != ref {
		t.Fatalf("FileReference() = %+v, want %+v", got.FileReference(), ref)
	}
	if !got.ObservedAt().Equal(observedAt) {
		t.Fatalf("ObservedAt() = %v, want %v", got.ObservedAt(), observedAt)
	}
}

// domain-source.md FR-2/FR-3: re-observing the same (Source, Edition,
// Format) tuple updates that row rather than inserting a second one —
// proven against the real UNIQUE constraint on
// (source_id, edition_id, file_reference_format), not just Go-level
// UniquenessKey() equality.
func TestSourceOfferingRepository_Save_ReobservingSameKeyUpdatesRow(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src', true, true, true, '')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewSourceOfferingRepository(pool)

	ref1, err := domain.NewFileReference("ref-1", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference ref1: %v", err)
	}
	firstObservedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	first := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref1, firstObservedAt)
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	size := int64(2048)
	ref2, err := domain.NewFileReference("ref-1-updated", "epub", &size)
	if err != nil {
		t.Fatalf("NewFileReference ref2: %v", err)
	}
	secondObservedAt := time.Now().UTC().Truncate(time.Microsecond)
	second := domain.NewSourceOffering("offering-2", "source-1", "edition-1", ref2, secondObservedAt)
	if err := repo.Save(ctx, second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	offerings, err := repo.FindBySource(ctx, "source-1")
	if err != nil {
		t.Fatalf("FindBySource: %v", err)
	}
	if len(offerings) != 1 {
		t.Fatalf("FindBySource = %d offerings, want 1 (re-observed row must update, not insert)", len(offerings))
	}
	got := offerings[0]
	gotRef := got.FileReference()
	if gotRef.ReferenceID != ref2.ReferenceID || gotRef.Format != ref2.Format ||
		gotRef.SizeBytes == nil || *gotRef.SizeBytes != *ref2.SizeBytes {
		t.Fatalf("FileReference() = %+v, want %+v (row not updated)", gotRef, ref2)
	}
	if !got.ObservedAt().Equal(secondObservedAt) {
		t.Fatalf("ObservedAt() = %v, want %v", got.ObservedAt(), secondObservedAt)
	}
}

// A different Format for the same (Source, Edition) is a distinct
// uniqueness key — it must insert a second row, not collide with the
// first.
func TestSourceOfferingRepository_Save_DifferentFormatInsertsNewRow(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src', true, true, true, '')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewSourceOfferingRepository(pool)

	refEpub, err := domain.NewFileReference("ref-epub", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference epub: %v", err)
	}
	epub := domain.NewSourceOffering("offering-epub", "source-1", "edition-1", refEpub, time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, epub); err != nil {
		t.Fatalf("Save epub: %v", err)
	}

	refPdf, err := domain.NewFileReference("ref-pdf", "pdf", nil)
	if err != nil {
		t.Fatalf("NewFileReference pdf: %v", err)
	}
	pdf := domain.NewSourceOffering("offering-pdf", "source-1", "edition-1", refPdf, time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, pdf); err != nil {
		t.Fatalf("Save pdf: %v", err)
	}

	offerings, err := repo.FindBySource(ctx, "source-1")
	if err != nil {
		t.Fatalf("FindBySource: %v", err)
	}
	if len(offerings) != 2 {
		t.Fatalf("FindBySource = %d offerings, want 2 (different Format must not collide)", len(offerings))
	}
}

func TestSourceOfferingRepository_FindByID_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewSourceOfferingRepository(pool)

	_, err := repo.FindByID(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestSourceOfferingRepository_FindBySource_ExcludesOtherSources(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src1', true, true, true, '')")
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-2', 'Src2', true, true, true, '')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewSourceOfferingRepository(pool)

	ref, err := domain.NewFileReference("ref-1", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	o1 := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref, now)
	o2 := domain.NewSourceOffering("offering-2", "source-2", "edition-1", ref, now)
	if err := repo.Save(ctx, o1); err != nil {
		t.Fatalf("Save o1: %v", err)
	}
	if err := repo.Save(ctx, o2); err != nil {
		t.Fatalf("Save o2: %v", err)
	}

	got, err := repo.FindBySource(ctx, "source-1")
	if err != nil {
		t.Fatalf("FindBySource: %v", err)
	}
	if len(got) != 1 || got[0].ID() != "offering-1" {
		t.Fatalf("FindBySource(source-1) = %v, want only offering-1", got)
	}
}

func TestSourceOfferingRepository_Delete(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src', true, true, true, '')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewSourceOfferingRepository(pool)

	ref, err := domain.NewFileReference("ref-1", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference: %v", err)
	}
	o := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref, time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, o); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, "offering-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.FindByID(ctx, "offering-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}
}

// Reuses R4's own SQL-injection proof mechanism against
// SourceOfferingRepository's own INSERT.
func TestSourceOfferingRepository_SQLInjectionProof(t *testing.T) {
	pool, tracer := tracedPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src', true, true, true, '')")
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	repo := postgres.NewSourceOfferingRepository(pool)

	hostileReferenceID := `O'Brien'; DROP TABLE source_offerings; --`
	ref, err := domain.NewFileReference(hostileReferenceID, "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference: %v", err)
	}
	o := domain.NewSourceOffering("offering-injection", "source-1", "edition-1", ref, time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Save(ctx, o); err != nil {
		t.Fatalf("Save with hostile file reference id: %v", err)
	}

	got, err := repo.FindByID(ctx, "offering-injection")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.FileReference().ReferenceID != hostileReferenceID {
		t.Fatalf("ReferenceID did not round-trip byte-for-byte: got %q, want %q", got.FileReference().ReferenceID, hostileReferenceID)
	}

	var tableExists bool
	if err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'source_offerings')",
	).Scan(&tableExists); err != nil {
		t.Fatalf("checking source_offerings table existence: %v", err)
	}
	if !tableExists {
		t.Fatal("source_offerings table no longer exists — DROP TABLE executed")
	}

	insertQueries := tracer.queriesContaining("INSERT INTO source_offerings")
	if len(insertQueries) == 0 {
		t.Fatal("no traced INSERT INTO source_offerings query — tracer wiring is broken")
	}
	for _, q := range insertQueries {
		if !strings.Contains(q.SQL, "$1") {
			t.Fatalf("traced SQL has no placeholder: %q", q.SQL)
		}
		if strings.Contains(q.SQL, hostileReferenceID) {
			t.Fatalf("hostile value interpolated directly into SQL text: %q", q.SQL)
		}
		found := false
		for _, arg := range q.Args {
			if s, ok := arg.(string); ok && s == hostileReferenceID {
				found = true
			}
		}
		if !found {
			t.Fatalf("hostile value not present in the query's separate argument list: %v", q.Args)
		}
	}
}
