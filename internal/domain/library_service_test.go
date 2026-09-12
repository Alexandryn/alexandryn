package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func mustNewEdition(t *testing.T, id domain.EditionID, workID domain.WorkID) *domain.Edition {
	t.Helper()
	lang, _ := domain.NewLanguage("en")
	e, err := domain.NewEdition(id, workID, lang, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition(%q): %v", id, err)
	}
	return e
}

// A LibraryEntry's existence does not depend on current availability,
// and creating one requires the referenced Edition to exist.
func TestLibraryService_AddEntry(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("adding an entry for an existing Edition succeeds", func(t *testing.T) {
		edition := mustNewEdition(t, "edition-1", "work-1")
		editions := newFakeEditionRepository(edition)
		entries := newFakeLibraryEntryRepository()
		svc := domain.NewLibraryService(editions, entries)

		entry, event, err := svc.AddEntry(ctx, "edition-1", now)
		if err != nil {
			t.Fatalf("AddEntry: %v", err)
		}
		if entry.EditionID() != "edition-1" {
			t.Fatalf("entry.EditionID() = %v, want edition-1", entry.EditionID())
		}
		if event == nil {
			t.Fatal("event = nil, want a LibraryEntryAdded for a genuinely new entry")
		}
	})

	t.Run("adding an entry for a nonexistent Edition is rejected", func(t *testing.T) {
		editions := newFakeEditionRepository()
		entries := newFakeLibraryEntryRepository()
		svc := domain.NewLibraryService(editions, entries)

		_, _, err := svc.AddEntry(ctx, "nonexistent-edition", now)
		if err == nil {
			t.Fatal("AddEntry(nonexistent) = nil error, want NotFound")
		}
		if domain.CategoryOf(err) != domain.NotFound {
			t.Fatalf("CategoryOf(err) = %v, want NotFound", domain.CategoryOf(err))
		}
	})

	// Adding an already-owned Edition again is an idempotent no-op.
	t.Run("adding the same Edition twice is a no-op, not a second entry", func(t *testing.T) {
		edition := mustNewEdition(t, "edition-1", "work-1")
		editions := newFakeEditionRepository(edition)
		entries := newFakeLibraryEntryRepository()
		svc := domain.NewLibraryService(editions, entries)

		_, firstEvent, err := svc.AddEntry(ctx, "edition-1", now)
		if err != nil {
			t.Fatalf("first AddEntry: %v", err)
		}
		if firstEvent == nil {
			t.Fatal("first AddEntry: event = nil, want LibraryEntryAdded")
		}

		secondEntry, secondEvent, err := svc.AddEntry(ctx, "edition-1", now.Add(time.Hour))
		if err != nil {
			t.Fatalf("second AddEntry: %v", err)
		}
		if secondEvent != nil {
			t.Fatal("second AddEntry: event != nil, want nil (no-op, nothing changed)")
		}
		if !secondEntry.AddedAt().Equal(now) {
			t.Fatalf("second AddEntry returned AddedAt = %v, want the original %v (no-op means unchanged)", secondEntry.AddedAt(), now)
		}
	})
}

// Removing a LibraryEntry does not cascade-delete the Work or
// Edition themselves, and does not touch Collection membership.
func TestLibraryService_RemoveEntry(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	edition := mustNewEdition(t, "edition-1", "work-1")
	editions := newFakeEditionRepository(edition)
	entries := newFakeLibraryEntryRepository()
	svc := domain.NewLibraryService(editions, entries)

	if _, _, err := svc.AddEntry(ctx, "edition-1", now); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	event, err := svc.RemoveEntry(ctx, "edition-1", now)
	if err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}
	if event == nil {
		t.Fatal("event = nil, want a LibraryEntryRemoved")
	}

	_, err = entries.FindByEdition(ctx, "edition-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("entry still exists after RemoveEntry, want NotFound on lookup")
	}

	// The Edition itself is untouched — still findable.
	if _, err := editions.FindByID(ctx, "edition-1"); err != nil {
		t.Fatalf("Edition was affected by RemoveEntry: %v", err)
	}
}

func TestLibraryService_RemoveEntry_NoExistingEntryIsANoOp(t *testing.T) {
	ctx := context.Background()
	editions := newFakeEditionRepository()
	entries := newFakeLibraryEntryRepository()
	svc := domain.NewLibraryService(editions, entries)

	event, err := svc.RemoveEntry(ctx, "edition-never-added", time.Now())
	if err != nil {
		t.Fatalf("RemoveEntry on nonexistent entry: %v", err)
	}
	if event != nil {
		t.Fatal("event != nil, want nil (no-op, nothing to remove)")
	}
}
