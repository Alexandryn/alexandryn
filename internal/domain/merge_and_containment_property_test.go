package domain_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Checkpoint P-H / domain-bibliographic.md's own Test strategy: "FR-8's
// invariants: no construction path produces a cycle... tested by
// generating adversarial inputs, not just hand-picked examples." Tier 2
// proved every *named* cycle shape (direct, 2-hop, 3-hop, merge-triggers-
// containment) by hand; this generates 500 randomized merge/containment
// operations across 6 Works and asserts the one property that actually
// matters: every Work's merge/containment graph stays resolvable —
// ResolveWork never fails — after every single operation, accepted or
// rejected. A surviving cycle would surface here as resolveCanonicalWork's
// own defensive "existing cycle" error, not a hang (it's bounded, not an
// infinite loop), so this test can never itself get stuck even if the
// property it's checking for turns out to be violated.
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
