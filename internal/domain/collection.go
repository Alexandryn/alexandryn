package domain

import "time"

// maxCollectionNameLength defines the maximum allowed length for a collection name.
const maxCollectionNameLength = 100

// CollectionMember is one Work's membership in a Collection, carrying its
// own added-at timestamp independent of any LibraryEntry's.
type CollectionMember struct {
	WorkID  WorkID
	AddedAt time.Time
}

// Collection contains Works, not Editions or files. An empty Collection is
// an ordinary initial state.
type Collection struct {
	id      CollectionID
	name    string
	members []CollectionMember
}

// NewCollection validates name against bounded-text constraints.
func NewCollection(id CollectionID, name string) (*Collection, error) {
	if err := ValidateBoundedText("name", name, maxCollectionNameLength); err != nil {
		return nil, err
	}
	return &Collection{id: id, name: name}, nil
}

func (c *Collection) ID() CollectionID { return c.id }

func (c *Collection) Name() string { return c.name }

func (c *Collection) Members() []CollectionMember { return c.members }

// AddMember adds a Work to the collection. It is a no-op if workID is already
// a member, ensuring idempotent addition.
func (c *Collection) AddMember(workID WorkID, addedAt time.Time) {
	for _, m := range c.members {
		if m.WorkID == workID {
			return
		}
	}
	c.members = append(c.members, CollectionMember{WorkID: workID, AddedAt: addedAt})
}

// RemoveMember removes exactly the given Work's membership without cascading.
func (c *Collection) RemoveMember(workID WorkID) {
	filtered := c.members[:0]
	for _, m := range c.members {
		if m.WorkID != workID {
			filtered = append(filtered, m)
		}
	}
	c.members = filtered
}
