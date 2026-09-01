package api

import (
	"context"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// ProgressStore is the row-locking slice of ReadingProgressRepository the
// reconcile transaction needs (backend-reading-api.md FR-2).
type ProgressStore interface {
	FindByWorkForUpdate(ctx context.Context, workID domain.WorkID) (*domain.ReadingProgress, error)
	Save(ctx context.Context, p *domain.ReadingProgress) error
}

// Transactor runs fn inside one transaction (ADR 0021).
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// IDs mints a new ReadingProgress row id for the first-report case.
type IDs interface {
	NewID() string
}

// ReportProgress runs FR-2's reconcile-and-persist atomically: inside one
// transaction it takes a row lock on the singleton ReadingProgress for
// the Work, folds the report through ReconcileProgress (or
// OverrideProgress when override is set), and persists only when the
// canonical value actually moved. The returned outcome tells the caller
// whether the report won.
func ReportProgress(ctx context.Context, tx Transactor, store ProgressStore, ids IDs, report domain.ProgressReport, override bool) (*domain.ReadingProgress, domain.ReconcileOutcome, error) {
	var result *domain.ReadingProgress
	var outcome domain.ReconcileOutcome

	err := tx.InTx(ctx, func(ctx context.Context) error {
		current, err := store.FindByWorkForUpdate(ctx, report.WorkID)
		if err != nil && domain.CategoryOf(err) == domain.NotFound {
			// No canonical value yet — this report becomes it directly
			// (State transitions), racing on work_id's UNIQUE constraint.
			first := domain.FirstProgress(domain.ReadingProgressID(ids.NewID()), report)
			if serr := store.Save(ctx, first); serr != nil {
				return serr
			}
			result, outcome = first, domain.ReconcileAdvanced
			return nil
		}
		if err != nil {
			return err
		}

		if override {
			next := domain.OverrideProgress(current, report.Percentage, report.PrecisePosition, report.DeviceID, report.ReportedAt)
			if serr := store.Save(ctx, next); serr != nil {
				return serr
			}
			result, outcome = next, domain.ReconcileOverridden
			return nil
		}

		res := domain.ReconcileProgress(current, report)
		result, outcome = res.Progress, res.Outcome
		if res.Outcome == domain.ReconcileAdvanced {
			return store.Save(ctx, res.Progress)
		}
		return nil // Unchanged / Rejected — no write
	})
	if err != nil {
		return nil, "", err
	}
	return result, outcome, nil
}

// ErrEditionWorkMismatch is returned by CheckPrecisePositionWork when the
// tagged Edition does not belong to the reported Work (FR-5).
var ErrEditionWorkMismatch = &domain.Error{Category: domain.InvalidInput, Message: "precisePosition.editionId does not belong to this work"}

// EditionLookup is the slice of EditionRepository FR-5's boundary check needs.
type EditionLookup interface {
	FindByID(ctx context.Context, id domain.EditionID) (*domain.Edition, error)
}

// CheckPrecisePositionWork restates domain-reading.md's illegal-transition
// rule at the transport boundary (FR-5): a PrecisePosition's tagged
// Edition MUST belong to the ReadingProgress's own Work.
func CheckPrecisePositionWork(ctx context.Context, editions EditionLookup, workID domain.WorkID, pos *domain.PrecisePosition) error {
	if pos == nil {
		return nil
	}
	edition, err := editions.FindByID(ctx, pos.EditionID)
	if err != nil {
		if domain.CategoryOf(err) == domain.NotFound {
			return ErrEditionWorkMismatch
		}
		return err
	}
	if edition.WorkID() != workID {
		return ErrEditionWorkMismatch
	}
	return nil
}
