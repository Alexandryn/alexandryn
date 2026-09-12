package domain

import "time"

// LibraryEntry represents ownership of a specific Edition (never a Work directly).
// Availability is maintained separately and does not determine library entry existence.
type LibraryEntry struct {
	id        LibraryEntryID
	editionID EditionID
	addedAt   time.Time
}

// NewLibraryEntry has no free-form fields to validate — both IDs are
// required positional arguments (no nil-able "missing Edition" state,
// same reasoning as Edition's own required WorkID).
func NewLibraryEntry(id LibraryEntryID, editionID EditionID, addedAt time.Time) *LibraryEntry {
	return &LibraryEntry{id: id, editionID: editionID, addedAt: addedAt}
}

func (e *LibraryEntry) ID() LibraryEntryID { return e.id }

func (e *LibraryEntry) EditionID() EditionID { return e.editionID }

func (e *LibraryEntry) AddedAt() time.Time { return e.addedAt }
