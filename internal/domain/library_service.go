package domain

import (
	"context"
	"time"
)

// LibraryService owns adding and removing a LibraryEntry
// (domain-library.md FR-1/FR-3/FR-6/FR-7), holding the repository
// interfaces internal/domain itself declares. The Edition-existence
// check (ADR 0020's own named example) and the at-most-one-per-Edition
// uniqueness check (FR-7) both require reading other records, so both
// live here rather than at construction.
type LibraryService struct {
	editions EditionRepository
	entries  LibraryEntryRepository
}

func NewLibraryService(editions EditionRepository, entries LibraryEntryRepository) *LibraryService {
	return &LibraryService{editions: editions, entries: entries}
}

// AddEntry creates a LibraryEntry for editionID, rejecting if the
// Edition doesn't exist. If an entry already exists for this Edition,
// this is a no-op (FR-7: "adding an already-owned Edition again is a
// no-op, not a second row") — the existing entry is returned unchanged
// and the returned event is nil, since nothing actually changed
// (FR-5: exactly one event per logical change, and a no-op is not one).
func (s *LibraryService) AddEntry(ctx context.Context, editionID EditionID, addedAt time.Time) (*LibraryEntry, *LibraryEntryAdded, error) {
	if _, err := s.editions.FindByID(ctx, editionID); err != nil {
		return nil, nil, err
	}

	existing, err := s.entries.FindByEdition(ctx, editionID)
	if err == nil {
		return existing, nil, nil
	}
	if CategoryOf(err) != NotFound {
		return nil, nil, err
	}

	entryID := LibraryEntryID(string(editionID)) // FR-7's uniqueness key is the Edition itself; the entry's own ID is derived from it, not independently generated, since at most one can ever exist per Edition.
	entry := NewLibraryEntry(entryID, editionID, addedAt)
	if err := s.entries.Save(ctx, entry); err != nil {
		return nil, nil, err
	}
	event := NewLibraryEntryAdded(string(entryID), addedAt)
	return entry, &event, nil
}

// RemoveEntry deletes the LibraryEntry for editionID. MUST NOT cascade to
// Work or Edition (FR-6) — this method never touches either repository.
// If no entry exists, this is a no-op (symmetric with AddEntry's own
// no-op case; domain-library.md doesn't name a required error here).
func (s *LibraryService) RemoveEntry(ctx context.Context, editionID EditionID, occurredAt time.Time) (*LibraryEntryRemoved, error) {
	existing, err := s.entries.FindByEdition(ctx, editionID)
	if err != nil {
		if CategoryOf(err) == NotFound {
			return nil, nil
		}
		return nil, err
	}
	if err := s.entries.DeleteByEdition(ctx, editionID); err != nil {
		return nil, err
	}
	event := NewLibraryEntryRemoved(string(existing.ID()), occurredAt)
	return &event, nil
}
