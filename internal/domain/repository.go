package domain

import "context"

// WorkRepository and AuthorRepository are declared by internal/domain,
// satisfied by internal/persistence/postgres (phase 03's T24) — the
// pattern ADR 0020 requires: a domain service performs IO through an
// interface the domain itself owns, never a persistence-specific type
// (go-backend-conventions/SKILL.md).
//
// Both are minimal interfaces (E4, tasks/plan-phase02-domain.md): only
// what WorkMergeService/AuthorMergeService/WorkContainmentService need to
// enforce their own invariants, not backend-persistence.md FR-2's full
// CRUD shape — phase 03's T24 will need a broader interface; reconciling
// the two is that task's job, not this one's.
//
// FindByID returns a *Error with category NotFound when id doesn't
// exist — callers rely on CategoryOf to distinguish "not found" from any
// other failure, never a sentinel error value.
type WorkRepository interface {
	FindByID(ctx context.Context, id WorkID) (*Work, error)

	// FindMergedInto returns every Work whose MergedInto directly equals
	// canonical (one hop, not transitive) — the primitive
	// WorkContainmentService needs to compute the merge-resolved
	// containment union FR-9's own amendment requires
	// (domain-bibliographic.md: "a read of a canonical Work's ...
	// containment references MUST return the union across that Work and
	// every Work merged into it, transitively").
	FindMergedInto(ctx context.Context, canonical WorkID) ([]*Work, error)

	Save(ctx context.Context, w *Work) error
}

type AuthorRepository interface {
	FindByID(ctx context.Context, id AuthorID) (*Author, error)
	Save(ctx context.Context, a *Author) error
}
