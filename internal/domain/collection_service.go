package domain

import (
	"context"
	"time"
)

// CollectionService owns Collection creation and deletion
// (domain-library.md FR-4/FR-8/FR-9/FR-10). Member add/remove stay plain
// Collection methods (no repository read needed to enforce them — see
// Collection.AddMember's own doc comment) — this service exists for
// Create (needs an IDGenerator and a place to persist) and Delete, and
// wraps member mutation only to load/save the aggregate and produce the
// matching event.
type CollectionService struct {
	collections CollectionRepository
	ids         IDGenerator
}

func NewCollectionService(collections CollectionRepository, ids IDGenerator) *CollectionService {
	return &CollectionService{collections: collections, ids: ids}
}

func (s *CollectionService) Create(ctx context.Context, libraryID LibraryID, name string, occurredAt time.Time) (*Collection, CollectionCreated, error) {
	id := CollectionID(s.ids.NewID())
	c, err := NewCollection(id, name)
	if err != nil {
		return nil, CollectionCreated{}, err
	}
	if err := s.collections.Save(ctx, libraryID, c); err != nil {
		return nil, CollectionCreated{}, err
	}
	return c, NewCollectionCreated(string(id), occurredAt), nil
}

// Delete removes the Collection itself — its membership records go with
// it, and nothing else does (FR-10): no Work, Edition, or LibraryEntry is
// ever touched, because this method never reaches those repositories at
// all.
func (s *CollectionService) Delete(ctx context.Context, libraryID LibraryID, id CollectionID) error {
	return s.collections.Delete(ctx, libraryID, id)
}

// AddMember loads the Collection, delegates to its own AddMember method
// (no ownership check — domain-library.md names none for Collection
// membership), and persists the result.
func (s *CollectionService) AddMember(ctx context.Context, libraryID LibraryID, collectionID CollectionID, workID WorkID, occurredAt time.Time) (CollectionMemberAdded, error) {
	c, err := s.collections.FindByID(ctx, libraryID, collectionID)
	if err != nil {
		return CollectionMemberAdded{}, err
	}
	c.AddMember(workID, occurredAt)
	if err := s.collections.Save(ctx, libraryID, c); err != nil {
		return CollectionMemberAdded{}, err
	}
	return NewCollectionMemberAdded(string(collectionID), occurredAt), nil
}

func (s *CollectionService) RemoveMember(ctx context.Context, libraryID LibraryID, collectionID CollectionID, workID WorkID, occurredAt time.Time) (CollectionMemberRemoved, error) {
	c, err := s.collections.FindByID(ctx, libraryID, collectionID)
	if err != nil {
		return CollectionMemberRemoved{}, err
	}
	c.RemoveMember(workID)
	if err := s.collections.Save(ctx, libraryID, c); err != nil {
		return CollectionMemberRemoved{}, err
	}
	return NewCollectionMemberRemoved(string(collectionID), occurredAt), nil
}
