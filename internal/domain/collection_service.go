package domain

import (
	"context"
	"time"
)

// CollectionService owns Collection creation, deletion, and member mutation lifecycle.
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

// Delete removes the Collection itself and its membership records.
// No Work, Edition, or LibraryEntry is modified.
func (s *CollectionService) Delete(ctx context.Context, libraryID LibraryID, id CollectionID) error {
	return s.collections.Delete(ctx, libraryID, id)
}

// AddMember loads the Collection, delegates to its own AddMember method,
// and persists the result.
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
