package domain

import (
	"context"
	"time"
)

// WorkMergeService owns recording and undoing a Work merge
// (domain-bibliographic.md FR-4/FR-8), holding the repository interface
// internal/domain itself declares — the graph invariant (no cycle by
// reachability) is enforced here, not at construction, per ADR 0020.
type WorkMergeService struct {
	works WorkRepository
}

func NewWorkMergeService(works WorkRepository) *WorkMergeService {
	return &WorkMergeService{works: works}
}

// RecordMerge sets source's MergedInto to target — "what a merge moves:
// nothing" (FR-4): no Edition, author, subject, external reference, or
// containment reference is copied, moved, or rewritten, only this one
// field. Rejected if source is already reachable from target by
// following merge references, of which source == target is only the
// shortest case (FR-8) — walking from target forward catches both the
// direct case and any longer indirect chain (review 0048 finding 9's
// exact fix: A merged into B, then B into A, must be rejected).
func (s *WorkMergeService) RecordMerge(ctx context.Context, source, target WorkID, occurredAt time.Time) (WorkMerged, error) {
	reachable, err := s.sourceReachableFromTarget(ctx, source, target)
	if err != nil {
		return WorkMerged{}, err
	}
	if reachable {
		return WorkMerged{}, &Error{Category: Conflict, Message: "recording this merge would create a cycle"}
	}

	selfContaining, err := s.wouldCreateSelfContainment(ctx, source, target)
	if err != nil {
		return WorkMerged{}, err
	}
	if selfContaining {
		return WorkMerged{}, &Error{Category: Conflict, Message: "recording this merge would make a Work's resolved containment reach itself"}
	}

	sourceWork, err := s.works.FindByID(ctx, source)
	if err != nil {
		return WorkMerged{}, err
	}
	sourceWork.mergedInto = &target
	if err := s.works.Save(ctx, sourceWork); err != nil {
		return WorkMerged{}, err
	}
	return NewWorkMerged(string(source), occurredAt), nil
}

// UndoMerge clears source's MergedInto — exactly the identity, since
// nothing else was ever moved (FR-4).
func (s *WorkMergeService) UndoMerge(ctx context.Context, source WorkID, occurredAt time.Time) (WorkMergeUndone, error) {
	sourceWork, err := s.works.FindByID(ctx, source)
	if err != nil {
		return WorkMergeUndone{}, err
	}
	sourceWork.mergedInto = nil
	if err := s.works.Save(ctx, sourceWork); err != nil {
		return WorkMergeUndone{}, err
	}
	return NewWorkMergeUndone(string(source), occurredAt), nil
}

// wouldCreateSelfContainment checks the case domain-bibliographic.md's
// own Open questions section resolved: A contains B (recorded legally,
// no cycle exists at that time), then B is merged into A. Checking raw
// references would miss it entirely — resolving turns it into a real
// cycle. Once source resolves to target (after this merge), any existing
// containment reference to source anywhere in target's own resolved
// closure will re-resolve to target itself. So the check is simply: does
// target's current containment closure already reach source's current
// canonical form?
func (s *WorkMergeService) wouldCreateSelfContainment(ctx context.Context, source, target WorkID) (bool, error) {
	targetCanonical, err := resolveCanonicalWork(ctx, s.works, target)
	if err != nil {
		return false, err
	}
	sourceCanonical, err := resolveCanonicalWork(ctx, s.works, source)
	if err != nil {
		return false, err
	}

	closure, err := containsClosure(ctx, s.works, targetCanonical)
	if err != nil {
		return false, err
	}
	return closure[sourceCanonical], nil
}

// sourceReachableFromTarget walks target's own merge chain forward
// (target.MergedInto, then that Work's MergedInto, ...) and reports
// whether it ever lands on source. If it does, setting
// source.MergedInto = target would close a cycle: target -> ... -> source
// -> target.
func (s *WorkMergeService) sourceReachableFromTarget(ctx context.Context, source, target WorkID) (bool, error) {
	current := target
	visited := map[WorkID]bool{}
	for {
		if current == source {
			return true, nil
		}
		if visited[current] {
			// An existing cycle already in the data, which RecordMerge's
			// own check should never have allowed to happen. Treat as
			// "not reachable further" rather than loop forever.
			return false, nil
		}
		visited[current] = true

		w, err := s.works.FindByID(ctx, current)
		if err != nil {
			return false, err
		}
		if w.MergedInto() == nil {
			return false, nil
		}
		current = *w.MergedInto()
	}
}
