package domain

import (
	"context"
	"time"
)

// AuthorMergeService manages author merges, providing the same reversibility
// and cycle-detection guarantees as WorkMergeService.
type AuthorMergeService struct {
	authors AuthorRepository
}

func NewAuthorMergeService(authors AuthorRepository) *AuthorMergeService {
	return &AuthorMergeService{authors: authors}
}

func (s *AuthorMergeService) RecordMerge(ctx context.Context, source, target AuthorID, occurredAt time.Time) (AuthorMerged, error) {
	reachable, err := s.sourceReachableFromTarget(ctx, source, target)
	if err != nil {
		return AuthorMerged{}, err
	}
	if reachable {
		return AuthorMerged{}, &Error{Category: Conflict, Message: "recording this merge would create a cycle"}
	}

	sourceAuthor, err := s.authors.FindByID(ctx, source)
	if err != nil {
		return AuthorMerged{}, err
	}
	sourceAuthor.mergedInto = &target
	if err := s.authors.Save(ctx, sourceAuthor); err != nil {
		return AuthorMerged{}, err
	}
	return NewAuthorMerged(string(source), occurredAt), nil
}

func (s *AuthorMergeService) UndoMerge(ctx context.Context, source AuthorID, occurredAt time.Time) (AuthorMergeUndone, error) {
	sourceAuthor, err := s.authors.FindByID(ctx, source)
	if err != nil {
		return AuthorMergeUndone{}, err
	}
	sourceAuthor.mergedInto = nil
	if err := s.authors.Save(ctx, sourceAuthor); err != nil {
		return AuthorMergeUndone{}, err
	}
	return NewAuthorMergeUndone(string(source), occurredAt), nil
}

func (s *AuthorMergeService) sourceReachableFromTarget(ctx context.Context, source, target AuthorID) (bool, error) {
	current := target
	visited := map[AuthorID]bool{}
	for {
		if current == source {
			return true, nil
		}
		if visited[current] {
			return false, nil
		}
		visited[current] = true

		a, err := s.authors.FindByID(ctx, current)
		if err != nil {
			return false, err
		}
		if a.MergedInto() == nil {
			return false, nil
		}
		current = *a.MergedInto()
	}
}
