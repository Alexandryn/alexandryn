package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// domain-reading.md's own construction-time wording ("a PrecisePosition
// whose tagged Edition does not belong to the ReadingProgress's own Work
// — checked at construction") was corrected by ADR 0020: an Edition's
// parent Work is stored on the Edition, so deciding this requires
// reading it — a domain-service operation, not a bare constructor call.
func TestReadingProgressService_AttachPrecisePosition(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	pct, _ := domain.NewPercentage(0.5)

	t.Run("an Edition belonging to the same Work is accepted", func(t *testing.T) {
		edition := mustNewEdition(t, "edition-1", "work-1")
		editions := newFakeEditionRepository(edition)
		svc := domain.NewReadingProgressService(editions)

		progress := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", now)
		if err := svc.AttachPrecisePosition(ctx, progress, domain.PrecisePosition{EditionID: "edition-1", Value: "loc-100"}); err != nil {
			t.Fatalf("AttachPrecisePosition: %v", err)
		}
		if progress.PrecisePosition() == nil || progress.PrecisePosition().Value != "loc-100" {
			t.Fatalf("PrecisePosition() = %v, want {edition-1, loc-100}", progress.PrecisePosition())
		}
	})

	// The spec's own required acceptance-criterion test: a
	// PrecisePosition tagged to an Edition of a *different* Work is
	// rejected.
	t.Run("an Edition belonging to a different Work is rejected", func(t *testing.T) {
		edition := mustNewEdition(t, "edition-1", "work-OTHER")
		editions := newFakeEditionRepository(edition)
		svc := domain.NewReadingProgressService(editions)

		progress := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", now)
		err := svc.AttachPrecisePosition(ctx, progress, domain.PrecisePosition{EditionID: "edition-1", Value: "loc-100"})
		if err == nil {
			t.Fatal("AttachPrecisePosition with mismatched Work = nil error, want an error")
		}
		if domain.CategoryOf(err) != domain.InvalidInput {
			t.Fatalf("CategoryOf(err) = %v, want InvalidInput", domain.CategoryOf(err))
		}
		if progress.PrecisePosition() != nil {
			t.Fatalf("PrecisePosition() = %v, want nil (rejected attach must not partially apply)", progress.PrecisePosition())
		}
	})

	t.Run("a nonexistent Edition is rejected", func(t *testing.T) {
		editions := newFakeEditionRepository()
		svc := domain.NewReadingProgressService(editions)

		progress := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", now)
		err := svc.AttachPrecisePosition(ctx, progress, domain.PrecisePosition{EditionID: "nonexistent", Value: "loc-1"})
		if err == nil {
			t.Fatal("AttachPrecisePosition with nonexistent Edition = nil error, want an error")
		}
		if domain.CategoryOf(err) != domain.NotFound {
			t.Fatalf("CategoryOf(err) = %v, want NotFound", domain.CategoryOf(err))
		}
	})
}
