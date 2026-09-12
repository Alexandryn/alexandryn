package domain

import (
	"context"
	"time"
)

// WorkMergeService owns recording and undoing a Work merge, holding the
// repository interface internal/domain declares — the graph invariant
// (no cycle by reachability) is enforced here.
type WorkMergeService struct {
	works WorkRepository
}

func NewWorkMergeService(works WorkRepository) *WorkMergeService {
	return &WorkMergeService{works: works}
}

// RecordMerge sets source's MergedInto to target — no Edition, author,
// subject, external reference, or containment reference is copied, moved,
// or rewritten, only this one field. Rejected if source is already reachable
// from target by following merge references, preventing cycles.
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

// UndoMerge clears source's MergedInto — exactly reversing the merge.
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

// wouldCreateSelfContainment checks whether merging source into target would
// create a self-containment cycle (e.g. if target already contains source,
// merging source into target causes target to contain itself).
// It verifies whether target's resolved containment closure reaches
// source's canonical form.
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
