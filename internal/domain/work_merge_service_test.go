package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func mustNewWork(t *testing.T, id domain.WorkID, title string) *domain.Work {
	t.Helper()
	w, err := domain.NewWork(id, title, "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork(%q): %v", id, err)
	}
	return w
}

func TestWorkMergeService_RecordMerge(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("a simple, unambiguous merge succeeds", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		b := mustNewWork(t, "work-b", "Title B")
		repo := newFakeWorkRepository(a, b)
		svc := domain.NewWorkMergeService(repo)

		event, err := svc.RecordMerge(ctx, "work-a", "work-b", now)
		if err != nil {
			t.Fatalf("RecordMerge: %v", err)
		}
		if event.AggregateID() != "work-a" {
			t.Fatalf("event.AggregateID() = %v, want work-a", event.AggregateID())
		}

		reloaded, err := repo.FindByID(ctx, "work-a")
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if reloaded.MergedInto() == nil || *reloaded.MergedInto() != "work-b" {
			t.Fatalf("MergedInto() = %v, want work-b", reloaded.MergedInto())
		}
	})

	t.Run("direct self-merge is rejected", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		repo := newFakeWorkRepository(a)
		svc := domain.NewWorkMergeService(repo)

		_, err := svc.RecordMerge(ctx, "work-a", "work-a", now)
		if err == nil {
			t.Fatal("RecordMerge(A, A) = nil error, want a cycle rejection")
		}
		if domain.CategoryOf(err) != domain.Conflict {
			t.Fatalf("CategoryOf(err) = %v, want Conflict", domain.CategoryOf(err))
		}
	})

	// Indirect cycle: A merged into B, then a later attempt to merge B
	// into A must be rejected — not just direct self-reference.
	t.Run("indirect cycle (A into B, then B into A) is rejected", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		b := mustNewWork(t, "work-b", "Title B")
		repo := newFakeWorkRepository(a, b)
		svc := domain.NewWorkMergeService(repo)

		if _, err := svc.RecordMerge(ctx, "work-a", "work-b", now); err != nil {
			t.Fatalf("RecordMerge(A, B): %v", err)
		}

		_, err := svc.RecordMerge(ctx, "work-b", "work-a", now)
		if err == nil {
			t.Fatal("RecordMerge(B, A) after A already merged into B = nil error, want a cycle rejection")
		}
		if domain.CategoryOf(err) != domain.Conflict {
			t.Fatalf("CategoryOf(err) = %v, want Conflict", domain.CategoryOf(err))
		}
	})

	t.Run("a longer indirect cycle (A into B, B into C, then C into A) is rejected", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		b := mustNewWork(t, "work-b", "Title B")
		c := mustNewWork(t, "work-c", "Title C")
		repo := newFakeWorkRepository(a, b, c)
		svc := domain.NewWorkMergeService(repo)

		if _, err := svc.RecordMerge(ctx, "work-a", "work-b", now); err != nil {
			t.Fatalf("RecordMerge(A, B): %v", err)
		}
		if _, err := svc.RecordMerge(ctx, "work-b", "work-c", now); err != nil {
			t.Fatalf("RecordMerge(B, C): %v", err)
		}

		_, err := svc.RecordMerge(ctx, "work-c", "work-a", now)
		if err == nil {
			t.Fatal("RecordMerge(C, A) closing a 3-hop cycle = nil error, want a cycle rejection")
		}
	})
}

func TestWorkMergeService_UndoMerge(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("undo is exactly the identity", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		b := mustNewWork(t, "work-b", "Title B")
		repo := newFakeWorkRepository(a, b)
		svc := domain.NewWorkMergeService(repo)

		if _, err := svc.RecordMerge(ctx, "work-a", "work-b", now); err != nil {
			t.Fatalf("RecordMerge: %v", err)
		}
		if _, err := svc.UndoMerge(ctx, "work-a", now); err != nil {
			t.Fatalf("UndoMerge: %v", err)
		}

		reloaded, err := repo.FindByID(ctx, "work-a")
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if reloaded.MergedInto() != nil {
			t.Fatalf("MergedInto() after undo = %v, want nil", reloaded.MergedInto())
		}
		if reloaded.Title() != "Title A" {
			t.Fatalf("Title() after undo = %q, want unchanged %q", reloaded.Title(), "Title A")
		}
	})

	// Reading through two levels of merge, then undoing the first,
	// per the spec's own acceptance criterion.
	t.Run("undo one hop of a multi-level chain leaves the rest intact", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		b := mustNewWork(t, "work-b", "Title B")
		c := mustNewWork(t, "work-c", "Title C")
		repo := newFakeWorkRepository(a, b, c)
		svc := domain.NewWorkMergeService(repo)

		if _, err := svc.RecordMerge(ctx, "work-a", "work-b", now); err != nil {
			t.Fatalf("RecordMerge(A, B): %v", err)
		}
		if _, err := svc.RecordMerge(ctx, "work-b", "work-c", now); err != nil {
			t.Fatalf("RecordMerge(B, C): %v", err)
		}
		if _, err := svc.UndoMerge(ctx, "work-a", now); err != nil {
			t.Fatalf("UndoMerge(A): %v", err)
		}

		reloadedA, _ := repo.FindByID(ctx, "work-a")
		reloadedB, _ := repo.FindByID(ctx, "work-b")
		if reloadedA.MergedInto() != nil {
			t.Fatalf("A.MergedInto() = %v, want nil (undone)", reloadedA.MergedInto())
		}
		if reloadedB.MergedInto() == nil || *reloadedB.MergedInto() != "work-c" {
			t.Fatalf("B.MergedInto() = %v, want work-c (untouched by A's undo)", reloadedB.MergedInto())
		}
	})
}
