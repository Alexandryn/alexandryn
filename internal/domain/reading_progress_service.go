package domain

import "context"

// ReadingProgressService owns attaching a PrecisePosition to a
// ReadingProgress (domain-reading.md FR-2), holding the repository
// interface internal/domain itself declares. The graph invariant — the
// tagged Edition must belong to the same Work as the ReadingProgress
// itself — requires reading the Edition, so it lives here rather than at
// construction (ADR 0020's own named example for this exact case).
type ReadingProgressService struct {
	editions EditionRepository
}

func NewReadingProgressService(editions EditionRepository) *ReadingProgressService {
	return &ReadingProgressService{editions: editions}
}

// AttachPrecisePosition rejects a position whose Edition doesn't exist,
// or whose Edition belongs to a different Work than progress's own — a
// PrecisePosition for an unrelated Work's Edition must never attach
// (domain-reading.md's own required acceptance-criterion test). A
// rejected attach never partially applies: the field is only set after
// both checks pass.
func (s *ReadingProgressService) AttachPrecisePosition(ctx context.Context, progress *ReadingProgress, position PrecisePosition) error {
	edition, err := s.editions.FindByID(ctx, position.EditionID)
	if err != nil {
		return err
	}
	if edition.WorkID() != progress.WorkID() {
		return &Error{Category: InvalidInput, Message: "precise position's edition does not belong to this reading progress's work"}
	}
	progress.precisePosition = &position
	return nil
}
