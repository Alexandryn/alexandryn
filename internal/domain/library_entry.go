package domain

import "time"

// LibraryEntry (domain-library.md FR-1) is the unit of "the user has
// this" — references exactly one Edition, never a Work directly. No
// availability field: that's read from domain-source.md's model, never
// duplicated here (FR-3: existence MUST NOT depend on availability).
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
