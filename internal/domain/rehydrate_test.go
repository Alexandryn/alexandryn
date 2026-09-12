package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// RehydrateWork is a repository's own "read from storage" path, distinct
// from NewWork's "construct and validate new user input" path — it
// accepts MergedInto/Contains directly, which NewWork does not expose
// (those fields are set only by WorkMergeService/WorkContainmentService).
// A repository reading a row back is
// reconstructing already-decided, already-validated state, not deciding
// it, so this doesn't reopen either service's cycle check on the write
// path — nothing outside internal/domain can call WorkMergeService's
// unexported field assignment, and Rehydrate doesn't perform that
// assignment either, it just builds a Work value with the given fields.
func TestRehydrateWork_SetsMergedIntoAndContains(t *testing.T) {
	target := domain.WorkID("work-b")
	w := domain.RehydrateWork(
		"work-a", "Title", "Subtitle",
		[]domain.AuthorID{"author-1"},
		nil, nil, nil,
		&target,
		[]domain.WorkID{"work-c", "work-d"},
	)

	if w.ID() != "work-a" {
		t.Fatalf("ID() = %v, want work-a", w.ID())
	}
	if w.MergedInto() == nil || *w.MergedInto() != "work-b" {
		t.Fatalf("MergedInto() = %v, want work-b", w.MergedInto())
	}
	if len(w.Contains()) != 2 {
		t.Fatalf("Contains() = %v, want 2 entries", w.Contains())
	}
}

func TestRehydrateAuthor_SetsMergedInto(t *testing.T) {
	target := domain.AuthorID("author-b")
	a := domain.RehydrateAuthor("author-a", "Name", nil, &target)

	if a.ID() != "author-a" {
		t.Fatalf("ID() = %v, want author-a", a.ID())
	}
	if a.MergedInto() == nil || *a.MergedInto() != "author-b" {
		t.Fatalf("MergedInto() = %v, want author-b", a.MergedInto())
	}
}

// RehydrateReadingProgress is a repository's own "read from storage"
// path for ReadingProgress — it accepts precisePosition directly, which
// NewReadingProgress does not expose (that field is normally set only by
// ReadingProgressService.AttachPrecisePosition, which requires an
// EditionRepository to verify the position belongs to the same Work —
// state a repository reading an already-validated row back doesn't need
// to re-verify).
func TestRehydrateReadingProgress_SetsPrecisePosition(t *testing.T) {
	pct, err := domain.NewPercentage(0.5)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	observedAt := time.Now().UTC()
	pos := &domain.PrecisePosition{EditionID: "edition-1", Value: "loc-100"}

	p := domain.RehydrateReadingProgress("progress-1", "work-1", pct, 0, pos, "device-1", observedAt)

	if p.ID() != "progress-1" {
		t.Fatalf("ID() = %v, want progress-1", p.ID())
	}
	if p.PrecisePosition() == nil || *p.PrecisePosition() != *pos {
		t.Fatalf("PrecisePosition() = %v, want %v", p.PrecisePosition(), pos)
	}
}
