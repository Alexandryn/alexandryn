package domain

import (
	"context"
	"time"
)

// WorkContainmentService owns adding and removing an omnibus "contains"
// reference (domain-bibliographic.md FR-9), holding the repository
// interface internal/domain itself declares. The cycle check runs over
// the merge-resolved graph, not the raw one: both container and containee
// are resolved to their canonical Work before comparison, and the
// existing containment closure is walked with the same resolution applied
// at every step — the same reasoning WorkMergeService applies to merge
// cycles, per ADR 0020.
type WorkContainmentService struct {
	works WorkRepository
}

func NewWorkContainmentService(works WorkRepository) *WorkContainmentService {
	return &WorkContainmentService{works: works}
}

// AddContains records that container contains containee. Rejected if the
// two resolve to the same canonical Work (direct or merge-equivalent
// self-containment), or if containee's own merge-resolved containment
// closure already reaches container's canonical form (an indirect cycle
// of any length).
func (s *WorkContainmentService) AddContains(ctx context.Context, container, containee WorkID, occurredAt time.Time) (WorkContainsAdded, error) {
	containerCanonical, err := resolveCanonicalWork(ctx, s.works, container)
	if err != nil {
		return WorkContainsAdded{}, err
	}
	containeeCanonical, err := resolveCanonicalWork(ctx, s.works, containee)
	if err != nil {
		return WorkContainsAdded{}, err
	}
	if containerCanonical == containeeCanonical {
		return WorkContainsAdded{}, &Error{Category: Conflict, Message: "a Work cannot contain itself"}
	}

	closure, err := containsClosure(ctx, s.works, containeeCanonical)
	if err != nil {
		return WorkContainsAdded{}, err
	}
	if closure[containerCanonical] {
		return WorkContainsAdded{}, &Error{Category: Conflict, Message: "adding this containment reference would create a cycle"}
	}

	containerWork, err := s.works.FindByID(ctx, container)
	if err != nil {
		return WorkContainsAdded{}, err
	}
	containerWork.contains = append(containerWork.contains, containee)
	if err := s.works.Save(ctx, containerWork); err != nil {
		return WorkContainsAdded{}, err
	}
	return NewWorkContainsAdded(string(container), occurredAt), nil
}

// RemoveContains removes exactly the given containee reference from
// container's raw Contains list — no cascade, matching FR-6's
// non-cascading reasoning for LibraryEntry removal, applied here.
func (s *WorkContainmentService) RemoveContains(ctx context.Context, container, containee WorkID, occurredAt time.Time) (WorkContainsRemoved, error) {
	containerWork, err := s.works.FindByID(ctx, container)
	if err != nil {
		return WorkContainsRemoved{}, err
	}
	filtered := containerWork.contains[:0]
	for _, id := range containerWork.contains {
		if id != containee {
			filtered = append(filtered, id)
		}
	}
	containerWork.contains = filtered
	if err := s.works.Save(ctx, containerWork); err != nil {
		return WorkContainsRemoved{}, err
	}
	return NewWorkContainsRemoved(string(container), occurredAt), nil
}
