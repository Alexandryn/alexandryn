package domain

// RehydrateWork and RehydrateAuthor reconstruct a Work/Author from
// previously-persisted data — a repository implementation's own "read
// from storage" path (T24, tasks/plan-t24-repositories.md), distinct
// from NewWork/NewAuthor's "construct and validate new input" path.
// Unlike the validating constructors, these accept MergedInto/Contains
// directly: those fields are normally set only by
// WorkMergeService/AuthorMergeService/WorkContainmentService (ADR 0020's
// own invariant, enforced by keeping the corresponding struct fields
// unexported), but a repository reading a row back is reconstructing
// state those services already decided and already persisted, not
// deciding it fresh — there is no cycle check to re-run on a read.
// Validation is intentionally skipped too: data read back from storage
// was already validated when first written.
//
// These are exported because a repository implementation lives in a
// different package (internal/persistence/postgres) and Go has no
// visibility level between "this package" and "everyone" — callers
// outside a trusted repository implementation should use
// NewWork/NewAuthor instead, and this comment is that boundary's only
// enforcement, the same limit every Go repository-pattern codebase
// accepts.
func RehydrateWork(
	id WorkID,
	title, subtitle string,
	authors []AuthorID,
	subjects []Subject,
	originalLanguage *Language,
	externalReferences []ExternalReference,
	mergedInto *WorkID,
	contains []WorkID,
) *Work {
	return &Work{
		id:                 id,
		title:              title,
		subtitle:           subtitle,
		authors:            authors,
		subjects:           subjects,
		originalLanguage:   originalLanguage,
		externalReferences: externalReferences,
		mergedInto:         mergedInto,
		contains:           contains,
	}
}

func RehydrateAuthor(
	id AuthorID,
	name string,
	externalReferences []ExternalReference,
	mergedInto *AuthorID,
) *Author {
	return &Author{
		id:                 id,
		name:               name,
		externalReferences: externalReferences,
		mergedInto:         mergedInto,
	}
}
