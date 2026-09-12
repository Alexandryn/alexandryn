package domain

import (
	"context"
	"time"
)

// LibraryService owns adding and removing a LibraryEntry, holding the
// repository interfaces internal/domain declares. The Edition-existence
// check and at-most-one-per-Edition uniqueness check both require reading
// persisted state, so both live here rather than at construction.
type LibraryService struct {
	editions EditionRepository
	entries  LibraryEntryRepository
}

func NewLibraryService(editions EditionRepository, entries LibraryEntryRepository) *LibraryService {
	return &LibraryService{editions: editions, entries: entries}
}

// AddEntry creates a LibraryEntry for editionID, rejecting if the
// Edition doesn't exist. If an entry already exists for this Edition,
// this is an idempotent no-op — the existing entry is returned unchanged
// and the returned event is nil, since nothing actually changed.
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

	entryID := LibraryEntryID(string(editionID)) // The uniqueness key is the Edition itself; at most one entry exists per Edition.
	entry := NewLibraryEntry(entryID, editionID, addedAt)
	if err := s.entries.Save(ctx, entry); err != nil {
		return nil, nil, err
	}
	event := NewLibraryEntryAdded(string(entryID), addedAt)
	return entry, &event, nil
}

// RemoveEntry deletes the LibraryEntry for editionID. Does not cascade to
// Work or Edition. If no entry exists, this is a no-op.
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
