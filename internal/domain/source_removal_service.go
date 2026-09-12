package domain

import (
	"context"
	"time"
)

// SourceRemovalService owns removing a Source: every SourceOffering referencing
// it is removed atomically, and no LibraryEntry is touched. The cascade applies
// atomically through the Transactor.
type SourceRemovalService struct {
	sources   SourceRepository
	offerings SourceOfferingRepository
	tx        Transactor
}

func NewSourceRemovalService(sources SourceRepository, offerings SourceOfferingRepository, tx Transactor) *SourceRemovalService {
	return &SourceRemovalService{sources: sources, offerings: offerings, tx: tx}
}

// Remove deletes every SourceOffering referencing sourceID, then the
// Source itself, all inside one Transactor.InTx call, returning one
// SourceOfferingRemoved event per offering removed plus the SourceRemoved
// event. If the transaction itself fails — to begin, or partway through
// — no partial deletion is visible: fn's own error (or a panic) causes
// the real implementation to roll back everything, and this method
// returns the error without having "succeeded" at only part of the
// cascade.
func (s *SourceRemovalService) Remove(ctx context.Context, sourceID SourceID, occurredAt time.Time) (SourceRemoved, []SourceOfferingRemoved, error) {
	var sourceEvent SourceRemoved
	var offeringEvents []SourceOfferingRemoved

	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		offerings, err := s.offerings.FindBySource(ctx, sourceID)
		if err != nil {
			return err
		}

		for _, o := range offerings {
			if err := s.offerings.Delete(ctx, o.ID()); err != nil {
				return err
			}
			offeringEvents = append(offeringEvents, NewSourceOfferingRemoved(string(o.ID()), occurredAt))
		}

		if err := s.sources.Delete(ctx, sourceID); err != nil {
			return err
		}
		sourceEvent = NewSourceRemoved(string(sourceID), occurredAt)
		return nil
	})
	if err != nil {
		return SourceRemoved{}, nil, err
	}
	return sourceEvent, offeringEvents, nil
}
