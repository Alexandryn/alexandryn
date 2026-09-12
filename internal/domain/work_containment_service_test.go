package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestWorkContainmentService_AddContains(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("a simple containment reference succeeds", func(t *testing.T) {
		omnibus := mustNewWork(t, "omnibus", "The Trilogy")
		vol1 := mustNewWork(t, "vol-1", "Volume 1")
		repo := newFakeWorkRepository(omnibus, vol1)
		svc := domain.NewWorkContainmentService(repo)

		if _, err := svc.AddContains(ctx, "omnibus", "vol-1", now); err != nil {
			t.Fatalf("AddContains: %v", err)
		}
		reloaded, _ := repo.FindByID(ctx, "omnibus")
		if len(reloaded.Contains()) != 1 || reloaded.Contains()[0] != "vol-1" {
			t.Fatalf("Contains() = %v, want [vol-1]", reloaded.Contains())
		}
	})

	t.Run("direct self-containment is rejected", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		repo := newFakeWorkRepository(a)
		svc := domain.NewWorkContainmentService(repo)

		_, err := svc.AddContains(ctx, "work-a", "work-a", now)
		if err == nil {
			t.Fatal("AddContains(A, A) = nil error, want a cycle rejection")
		}
		if domain.CategoryOf(err) != domain.Conflict {
			t.Fatalf("CategoryOf(err) = %v, want Conflict", domain.CategoryOf(err))
		}
	})

	t.Run("indirect containment cycle (A contains B, B contains C, then C contains A) is rejected", func(t *testing.T) {
		a := mustNewWork(t, "work-a", "Title A")
		b := mustNewWork(t, "work-b", "Title B")
		c := mustNewWork(t, "work-c", "Title C")
		repo := newFakeWorkRepository(a, b, c)
		svc := domain.NewWorkContainmentService(repo)

		if _, err := svc.AddContains(ctx, "work-a", "work-b", now); err != nil {
			t.Fatalf("AddContains(A, B): %v", err)
		}
		if _, err := svc.AddContains(ctx, "work-b", "work-c", now); err != nil {
			t.Fatalf("AddContains(B, C): %v", err)
		}

		_, err := svc.AddContains(ctx, "work-c", "work-a", now)
		if err == nil {
			t.Fatal("AddContains(C, A) closing a 3-hop cycle = nil error, want a cycle rejection")
		}
	})

	t.Run("RemoveContains removes only the given reference", func(t *testing.T) {
		omnibus := mustNewWork(t, "omnibus", "The Trilogy")
		vol1 := mustNewWork(t, "vol-1", "Volume 1")
		vol2 := mustNewWork(t, "vol-2", "Volume 2")
		repo := newFakeWorkRepository(omnibus, vol1, vol2)
		svc := domain.NewWorkContainmentService(repo)

		if _, err := svc.AddContains(ctx, "omnibus", "vol-1", now); err != nil {
			t.Fatalf("AddContains(omnibus, vol-1): %v", err)
		}
		if _, err := svc.AddContains(ctx, "omnibus", "vol-2", now); err != nil {
			t.Fatalf("AddContains(omnibus, vol-2): %v", err)
		}
		if _, err := svc.RemoveContains(ctx, "omnibus", "vol-1", now); err != nil {
			t.Fatalf("RemoveContains: %v", err)
		}

		reloaded, _ := repo.FindByID(ctx, "omnibus")
		if len(reloaded.Contains()) != 1 || reloaded.Contains()[0] != "vol-2" {
			t.Fatalf("Contains() after removal = %v, want [vol-2]", reloaded.Contains())
		}
	})
}

// TestWorkMergeService_RejectsAMergeThatWouldCreateASelfContainmentCycle verifies
// that when A contains B, merging B into A is rejected because it would create
// a self-containment cycle.
func TestWorkMergeService_RejectsAMergeThatWouldCreateASelfContainmentCycle(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	a := mustNewWork(t, "work-a", "Title A")
	b := mustNewWork(t, "work-b", "Title B")
	repo := newFakeWorkRepository(a, b)
	containmentSvc := domain.NewWorkContainmentService(repo)
	mergeSvc := domain.NewWorkMergeService(repo)

	if _, err := containmentSvc.AddContains(ctx, "work-a", "work-b", now); err != nil {
		t.Fatalf("AddContains(A, B): %v", err)
	}

	_, err := mergeSvc.RecordMerge(ctx, "work-b", "work-a", now)
	if err == nil {
		t.Fatal("RecordMerge(B, A) after A already contains B = nil error, want a self-containment rejection")
	}
	if domain.CategoryOf(err) != domain.Conflict {
		t.Fatalf("CategoryOf(err) = %v, want Conflict", domain.CategoryOf(err))
	}

	// The attempted merge must not have partially applied.
	reloadedB, _ := repo.FindByID(ctx, "work-b")
	if reloadedB.MergedInto() != nil {
		t.Fatalf("B.MergedInto() = %v, want nil (rejected merge must not partially apply)", reloadedB.MergedInto())
	}
}
