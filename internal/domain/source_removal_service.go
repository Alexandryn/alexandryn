package domain

import (
	"context"
	"time"
)

// SourceRemovalService owns removing a Source (domain-source.md FR-6):
// every SourceOffering referencing it MUST be removed, and no
// LibraryEntry MUST be touched. The second half of that guarantee is
// structural, not a check this service performs — it never holds a
// LibraryEntryRepository at all, so there is nothing here that could
// reach one.
type SourceRemovalService struct {
	sources   SourceRepository
	offerings SourceOfferingRepository
}

func NewSourceRemovalService(sources SourceRepository, offerings SourceOfferingRepository) *SourceRemovalService {
	return &SourceRemovalService{sources: sources, offerings: offerings}
}

// Remove deletes every SourceOffering referencing sourceID, then the
// Source itself, returning one SourceOfferingRemoved event per offering
// removed plus the SourceRemoved event.
func (s *SourceRemovalService) Remove(ctx context.Context, sourceID SourceID, occurredAt time.Time) (SourceRemoved, []SourceOfferingRemoved, error) {
	offerings, err := s.offerings.FindBySource(ctx, sourceID)
	if err != nil {
		return SourceRemoved{}, nil, err
	}

	var offeringEvents []SourceOfferingRemoved
	for _, o := range offerings {
		if err := s.offerings.Delete(ctx, o.ID()); err != nil {
			return SourceRemoved{}, nil, err
		}
		offeringEvents = append(offeringEvents, NewSourceOfferingRemoved(string(o.ID()), occurredAt))
	}

	if err := s.sources.Delete(ctx, sourceID); err != nil {
		return SourceRemoved{}, nil, err
	}

	return NewSourceRemoved(string(sourceID), occurredAt), offeringEvents, nil
}
