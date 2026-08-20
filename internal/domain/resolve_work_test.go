package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// FR-4: a read of a canonical Work's authors/subjects/external
// references/containment references MUST return the union across that
// Work and every Work merged into it, transitively — a three-level merge
// chain, reading the canonical Work's resolved view returns the union,
// not just the canonical row's own fields (spec's own required fixture).
func TestResolveWork_UnionsAcrossAThreeLevelMergeChain(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	subjA, _ := domain.NewSubject("Science Fiction")
	subjB, _ := domain.NewSubject("Utopian Fiction")
	subjC, _ := domain.NewSubject("Anarchism")

	a, _ := domain.NewWork("work-a", "Title A", "", []domain.AuthorID{"author-a"}, []domain.Subject{subjA}, nil, nil)
	b, _ := domain.NewWork("work-b", "Title B", "", []domain.AuthorID{"author-b"}, []domain.Subject{subjB}, nil, nil)
	c, _ := domain.NewWork("work-c", "Title C", "", []domain.AuthorID{"author-c"}, []domain.Subject{subjC}, nil, nil)
	repo := newFakeWorkRepository(a, b, c)
	svc := domain.NewWorkMergeService(repo)

	// C merged into B, B merged into A: A is canonical.
	if _, err := svc.RecordMerge(ctx, "work-c", "work-b", now); err != nil {
		t.Fatalf("RecordMerge(C, B): %v", err)
	}
	if _, err := svc.RecordMerge(ctx, "work-b", "work-a", now); err != nil {
		t.Fatalf("RecordMerge(B, A): %v", err)
	}

	resolved, err := domain.ResolveWork(ctx, repo, "work-a")
	if err != nil {
		t.Fatalf("ResolveWork(A): %v", err)
	}
	if resolved.CanonicalID != "work-a" {
		t.Fatalf("CanonicalID = %v, want work-a", resolved.CanonicalID)
	}
	if len(resolved.Authors) != 3 {
		t.Fatalf("Authors = %v, want 3 (union of A, B, C)", resolved.Authors)
	}
	if len(resolved.Subjects) != 3 {
		t.Fatalf("Subjects = %v, want 3 (union of A, B, C)", resolved.Subjects)
	}

	// Reading through a non-canonical member of the chain (B) resolves to
	// the same canonical union.
	resolvedFromB, err := domain.ResolveWork(ctx, repo, "work-b")
	if err != nil {
		t.Fatalf("ResolveWork(B): %v", err)
	}
	if resolvedFromB.CanonicalID != "work-a" {
		t.Fatalf("ResolveWork(B).CanonicalID = %v, want work-a", resolvedFromB.CanonicalID)
	}
	if len(resolvedFromB.Authors) != 3 {
		t.Fatalf("ResolveWork(B).Authors = %v, want 3", resolvedFromB.Authors)
	}
}

func TestResolveWork_UnmergedWorkResolvesToItself(t *testing.T) {
	ctx := context.Background()
	a, _ := domain.NewWork("work-a", "Title A", "", nil, nil, nil, nil)
	repo := newFakeWorkRepository(a)

	resolved, err := domain.ResolveWork(ctx, repo, "work-a")
	if err != nil {
		t.Fatalf("ResolveWork: %v", err)
	}
	if resolved.CanonicalID != "work-a" {
		t.Fatalf("CanonicalID = %v, want work-a", resolved.CanonicalID)
	}
}
