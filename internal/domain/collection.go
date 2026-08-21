package domain

import "time"

// maxCollectionNameLength is a reasoned placeholder (no spec gives a
// number), matching Subject's own bound — a short label, not a
// description.
const maxCollectionNameLength = 100

// CollectionMember is one Work's membership in a Collection, carrying its
// own added-at timestamp independent of any LibraryEntry's (FR-9) — you
// can want a book before or after you own it.
type CollectionMember struct {
	WorkID  WorkID
	AddedAt time.Time
}

// Collection (domain-library.md FR-4/FR-8) contains Works, not Editions
// or files. Legally empty at construction (FR-8) — an empty Collection is
// an ordinary state, not an error.
type Collection struct {
	id      CollectionID
	name    string
	members []CollectionMember
}

// NewCollection validates name against validate.go's bounded-text check
// — a collection name is exactly the same class of hostile-input-adjacent
// string a title is.
func NewCollection(id CollectionID, name string) (*Collection, error) {
	if err := ValidateBoundedText("name", name, maxCollectionNameLength); err != nil {
		return nil, err
	}
	return &Collection{id: id, name: name}, nil
}

func (c *Collection) ID() CollectionID { return c.id }

func (c *Collection) Name() string { return c.name }

func (c *Collection) Members() []CollectionMember { return c.members }

// AddMember is a plain aggregate operation, not a domain service: unlike
// LibraryEntry creation (which must verify the Edition exists,
// ADR 0020), domain-library.md names no equivalent existence check for a
// Collection's Work members — FR-4/FR-9 describe membership shape only.
// A no-op if workID is already a member (matching LibraryEntry's own
// at-most-once reasoning, FR-7, applied here for the same "add it again,
// nothing visible changes" UX).
func (c *Collection) AddMember(workID WorkID, addedAt time.Time) {
	for _, m := range c.members {
		if m.WorkID == workID {
			return
		}
	}
	c.members = append(c.members, CollectionMember{WorkID: workID, AddedAt: addedAt})
}

// RemoveMember removes exactly the given Work's membership — no cascade,
// same non-cascading reasoning FR-6 applies to removing a LibraryEntry.
func (c *Collection) RemoveMember(workID WorkID) {
	filtered := c.members[:0]
	for _, m := range c.members {
		if m.WorkID != workID {
			filtered = append(filtered, m)
		}
	}
	c.members = filtered
}
