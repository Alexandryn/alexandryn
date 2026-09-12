package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestIsInLibrary(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("Work with no owned Editions is not in the library", func(t *testing.T) {
		work := mustNewWork(t, "work-1", "Title")
		works := newFakeWorkRepository(work)
		editions := newFakeEditionRepository()
		entries := newFakeLibraryEntryRepository()

		inLibrary, err := domain.IsInLibrary(ctx, works, editions, entries, "work-1")
		if err != nil {
			t.Fatalf("IsInLibrary: %v", err)
		}
		if inLibrary {
			t.Fatal("IsInLibrary = true, want false (no LibraryEntry exists)")
		}
	})

	t.Run("Work with a directly owned Edition is in the library", func(t *testing.T) {
		work := mustNewWork(t, "work-1", "Title")
		edition := mustNewEdition(t, "edition-1", "work-1")
		works := newFakeWorkRepository(work)
		editions := newFakeEditionRepository(edition)
		entries := newFakeLibraryEntryRepository(domain.NewLibraryEntry("entry-1", "edition-1", now))

		inLibrary, err := domain.IsInLibrary(ctx, works, editions, entries, "work-1")
		if err != nil {
			t.Fatalf("IsInLibrary: %v", err)
		}
		if !inLibrary {
			t.Fatal("IsInLibrary = false, want true (edition-1 has a LibraryEntry)")
		}
	})

	// "In library" is computed over the merge-resolved Edition set.
	// A Work merged into another still answers true for "in library"
	// through the resolved view.
	t.Run("a Work merged away still resolves to true via its canonical Work's owned Edition", func(t *testing.T) {
		mergedAway := mustNewWork(t, "work-a", "Title A")
		canonical := mustNewWork(t, "work-b", "Title B")
		// The owned Edition belongs to the *canonical* Work, work-b —
		// work-a (the merged-away Work) owns nothing directly itself.
		edition := mustNewEdition(t, "edition-1", "work-b")

		works := newFakeWorkRepository(mergedAway, canonical)
		editions := newFakeEditionRepository(edition)
		entries := newFakeLibraryEntryRepository(domain.NewLibraryEntry("entry-1", "edition-1", now))
		mergeSvc := domain.NewWorkMergeService(works)

		if _, err := mergeSvc.RecordMerge(ctx, "work-a", "work-b", now); err != nil {
			t.Fatalf("RecordMerge(A, B): %v", err)
		}

		// Asking "is work-a in the library" — work-a itself owns no
		// Edition, but it resolves to work-b, which does.
		inLibrary, err := domain.IsInLibrary(ctx, works, editions, entries, "work-a")
		if err != nil {
			t.Fatalf("IsInLibrary(work-a): %v", err)
		}
		if !inLibrary {
			t.Fatal("IsInLibrary(work-a) = false, want true — work-a resolves to work-b, which owns edition-1")
		}
	})
}
