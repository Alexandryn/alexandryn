package domain_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// RehydrateWork is a repository's own "read from storage" path, distinct
// from NewWork's "construct and validate new user input" path — it
// accepts MergedInto/Contains directly, which NewWork does not expose
// (those fields are set only by WorkMergeService/WorkContainmentService,
// ADR 0020's own invariant). A repository reading a row back is
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
