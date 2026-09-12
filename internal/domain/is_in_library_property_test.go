package domain_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Property-based test proving that "Work in library" always matches
// "at least one Edition has an entry" for any generated sequence of
// entry creations/removals.
func TestIsInLibrary_ComputedNotStoredInvariant_PropertyBased(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(20260820))

	const workCount = 4
	const editionsPerWork = 3
	const operations = 200

	work := mustNewWork(t, "work-1", "Title")
	works := newFakeWorkRepository(work)

	var editionIDs []domain.EditionID
	editions := newFakeEditionRepository()
	for i := 0; i < editionsPerWork; i++ {
		id := domain.EditionID(fmt.Sprintf("edition-%d", i))
		editionIDs = append(editionIDs, id)
		e := mustNewEdition(t, id, "work-1")
		editions.editions[id] = e
	}
	_ = workCount // only one Work is exercised here; IsInLibrary's merge-resolution path has its own dedicated test above

	entries := newFakeLibraryEntryRepository()
	libSvc := domain.NewLibraryService(editions, entries)

	// ownedByUs tracks the model's own belief about which Editions
	// currently have an entry, checked against the real repository state
	// after every operation.
	ownedByUs := map[domain.EditionID]bool{}

	for op := 0; op < operations; op++ {
		edition := editionIDs[rng.Intn(len(editionIDs))]
		now := time.Now()

		if rng.Intn(2) == 0 {
			if _, _, err := libSvc.AddEntry(ctx, edition, now); err != nil {
				t.Fatalf("op %d: AddEntry(%s): %v", op, edition, err)
			}
			ownedByUs[edition] = true
		} else {
			if _, err := libSvc.RemoveEntry(ctx, edition, now); err != nil {
				t.Fatalf("op %d: RemoveEntry(%s): %v", op, edition, err)
			}
			ownedByUs[edition] = false
		}

		wantInLibrary := false
		for _, owned := range ownedByUs {
			if owned {
				wantInLibrary = true
				break
			}
		}

		gotInLibrary, err := domain.IsInLibrary(ctx, works, editions, entries, "work-1")
		if err != nil {
			t.Fatalf("op %d: IsInLibrary: %v", op, err)
		}
		if gotInLibrary != wantInLibrary {
			t.Fatalf("op %d: IsInLibrary = %v, want %v (model state: %+v)", op, gotInLibrary, wantInLibrary, ownedByUs)
		}
	}
}
