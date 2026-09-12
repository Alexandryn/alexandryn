package domain_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// TestWorkMergeAndContainment_NoCycleSurvivesRandomizedOperations verifies that
// randomized sequences of merge and containment operations never produce an
// unresolvable cycle in the work graph. It executes 500 randomized operations
// across 6 works and asserts that ResolveWork remains successful for all works
// after every accepted or rejected operation.
func TestWorkMergeAndContainment_NoCycleSurvivesRandomizedOperations(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(20260821))

	const workCount = 6
	const operations = 500

	var works []*domain.Work
	for i := 0; i < workCount; i++ {
		works = append(works, mustNewWork(t, domain.WorkID(fmt.Sprintf("work-%d", i)), fmt.Sprintf("Title %d", i)))
	}
	repo := newFakeWorkRepository(works...)
	mergeSvc := domain.NewWorkMergeService(repo)
	containSvc := domain.NewWorkContainmentService(repo)

	for op := 0; op < operations; op++ {
		a := domain.WorkID(fmt.Sprintf("work-%d", rng.Intn(workCount)))
		b := domain.WorkID(fmt.Sprintf("work-%d", rng.Intn(workCount)))
		now := time.Now()

		// Errors are expected and correct here — a cycle-forming
		// operation SHOULD be rejected. What must never happen is the
		// graph becoming unresolvable afterward, checked below
		// regardless of whether this particular operation succeeded.
		if rng.Intn(2) == 0 {
			_, _ = mergeSvc.RecordMerge(ctx, a, b, now)
		} else {
			_, _ = containSvc.AddContains(ctx, a, b, now)
		}

		for _, w := range works {
			if _, err := domain.ResolveWork(ctx, repo, w.ID()); err != nil {
				t.Fatalf("op %d (a=%s, b=%s): ResolveWork(%s) failed, possible undetected cycle: %v", op, a, b, w.ID(), err)
			}
		}
	}
}
