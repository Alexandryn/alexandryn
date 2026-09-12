//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestLibraryEntryRepository_SaveAndFindByEdition_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language) VALUES ('edition-1', 'work-1', 'en')")
	repo := postgres.NewLibraryEntryRepository(pool)

	addedAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	entry := domain.NewLibraryEntry("entry-1", "edition-1", addedAt)

	if err := repo.Save(ctx, entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByEdition(ctx, "edition-1")
	if err != nil {
		t.Fatalf("FindByEdition: %v", err)
	}
	if got.ID() != "entry-1" {
		t.Fatalf("ID() = %v, want entry-1", got.ID())
	}
	if got.EditionID() != "edition-1" {
		t.Fatalf("EditionID() = %v, want edition-1", got.EditionID())
	}
	if !got.AddedAt().Equal(addedAt) {
		t.Fatalf("AddedAt() = %v, want %v", got.AddedAt(), addedAt)
	}
}

func TestLibraryEntryRepository_FindByEdition_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewLibraryEntryRepository(pool)

	_, err := repo.FindByEdition(context.Background(), "nonexistent")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestLibraryEntryRepository_DeleteByEdition(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language) VALUES ('edition-1', 'work-1', 'en')")
	repo := postgres.NewLibraryEntryRepository(pool)

	entry := domain.NewLibraryEntry("entry-1", "edition-1", time.Now())
	if err := repo.Save(ctx, entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.DeleteByEdition(ctx, "edition-1"); err != nil {
		t.Fatalf("DeleteByEdition: %v", err)
	}

	_, err := repo.FindByEdition(ctx, "edition-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}
}

// At most one LibraryEntry per Edition — proven
// under real concurrent load, not just sequentially. Two goroutines Save
// distinct LibraryEntry values for the same EditionID at the same time;
// the real UNIQUE constraint on library_entries.edition_id must let
// exactly one through and translate the other's failure to Conflict —
// this is the repository-level version of the raw constraint proof
// schema_integration_test.go already has
// (TestSchema_LibraryEntriesUniqueByEdition), driven through Save
// itself and through two genuinely concurrent connections rather than
// one serialized test goroutine.
func TestLibraryEntryRepository_Save_ConcurrentSameEditionRace(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language) VALUES ('edition-1', 'work-1', 'en')")
	repo := postgres.NewLibraryEntryRepository(pool)

	entryA := domain.NewLibraryEntry("entry-a", "edition-1", time.Now())
	entryB := domain.NewLibraryEntry("entry-b", "edition-1", time.Now())

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = repo.Save(ctx, entryA)
	}()
	go func() {
		defer wg.Done()
		errs[1] = repo.Save(ctx, entryB)
	}()
	wg.Wait()

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case domain.CategoryOf(err) == domain.Conflict:
			conflicts++
		default:
			t.Fatalf("unexpected error category %v: %v", domain.CategoryOf(err), err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want exactly 1 and 1", successes, conflicts)
	}
}
