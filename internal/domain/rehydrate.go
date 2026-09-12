package domain

import "time"

// RehydrateWork and RehydrateAuthor reconstruct a Work/Author from
// previously-persisted data — a repository implementation's "read
// from storage" path, distinct from NewWork/NewAuthor's "construct and
// validate new input" path.
// Unlike the validating constructors, these accept MergedInto/Contains
// directly: those fields are normally set only by
// WorkMergeService/AuthorMergeService/WorkContainmentService (enforced
// by keeping the corresponding struct fields
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

// RehydrateReadingProgress reconstructs a ReadingProgress from a
// previously-persisted row — a repository's own "read from storage"
// path, distinct from NewReadingProgress's "construct new" path. Unlike
// NewReadingProgress, it accepts precisePosition directly: that field is
// normally set only by ReadingProgressService.AttachPrecisePosition,
// which verifies the position's Edition belongs to the same Work (a
// check requiring an EditionRepository) before assigning it — a
// repository reading an already-validated row back is reconstructing
// that decision, not making it again.
func RehydrateReadingProgress(
	id ReadingProgressID,
	workID WorkID,
	percentage Percentage,
	epoch int64,
	precisePosition *PrecisePosition,
	deviceID DeviceID,
	observedAt time.Time,
) *ReadingProgress {
	return &ReadingProgress{
		id:              id,
		workID:          workID,
		percentage:      percentage,
		epoch:           epoch,
		precisePosition: precisePosition,
		deviceID:        deviceID,
		observedAt:      observedAt,
	}
}
