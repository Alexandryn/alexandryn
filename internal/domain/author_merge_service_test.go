package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func mustNewAuthor(t *testing.T, id domain.AuthorID, name string) *domain.Author {
	t.Helper()
	a, err := domain.NewAuthor(id, name, nil)
	if err != nil {
		t.Fatalf("NewAuthor(%q): %v", id, err)
	}
	return a
}

// AuthorMergeService follows the same pattern as WorkMergeService,
// providing representative test coverage for author merge operations.
func TestAuthorMergeService_RecordMerge(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("a simple merge succeeds", func(t *testing.T) {
		a := mustNewAuthor(t, "author-a", "Name A")
		b := mustNewAuthor(t, "author-b", "Name B")
		repo := newFakeAuthorRepository(a, b)
		svc := domain.NewAuthorMergeService(repo)

		if _, err := svc.RecordMerge(ctx, "author-a", "author-b", now); err != nil {
			t.Fatalf("RecordMerge: %v", err)
		}
		reloaded, _ := repo.FindByID(ctx, "author-a")
		if reloaded.MergedInto() == nil || *reloaded.MergedInto() != "author-b" {
			t.Fatalf("MergedInto() = %v, want author-b", reloaded.MergedInto())
		}
	})

	t.Run("direct self-merge is rejected", func(t *testing.T) {
		a := mustNewAuthor(t, "author-a", "Name A")
		repo := newFakeAuthorRepository(a)
		svc := domain.NewAuthorMergeService(repo)

		_, err := svc.RecordMerge(ctx, "author-a", "author-a", now)
		if err == nil {
			t.Fatal("RecordMerge(A, A) = nil error, want a cycle rejection")
		}
	})

	t.Run("indirect cycle (A into B, then B into A) is rejected", func(t *testing.T) {
		a := mustNewAuthor(t, "author-a", "Name A")
		b := mustNewAuthor(t, "author-b", "Name B")
		repo := newFakeAuthorRepository(a, b)
		svc := domain.NewAuthorMergeService(repo)

		if _, err := svc.RecordMerge(ctx, "author-a", "author-b", now); err != nil {
			t.Fatalf("RecordMerge(A, B): %v", err)
		}
		_, err := svc.RecordMerge(ctx, "author-b", "author-a", now)
		if err == nil {
			t.Fatal("RecordMerge(B, A) after A already merged into B = nil error, want a cycle rejection")
		}
	})
}

func TestAuthorMergeService_UndoMerge(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	a := mustNewAuthor(t, "author-a", "Name A")
	b := mustNewAuthor(t, "author-b", "Name B")
	repo := newFakeAuthorRepository(a, b)
	svc := domain.NewAuthorMergeService(repo)

	if _, err := svc.RecordMerge(ctx, "author-a", "author-b", now); err != nil {
		t.Fatalf("RecordMerge: %v", err)
	}
	if _, err := svc.UndoMerge(ctx, "author-a", now); err != nil {
		t.Fatalf("UndoMerge: %v", err)
	}

	reloaded, _ := repo.FindByID(ctx, "author-a")
	if reloaded.MergedInto() != nil {
		t.Fatalf("MergedInto() after undo = %v, want nil", reloaded.MergedInto())
	}
	if reloaded.Name() != "Name A" {
		t.Fatalf("Name() after undo = %q, want unchanged %q", reloaded.Name(), "Name A")
	}
}
