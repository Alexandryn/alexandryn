package domain

import (
	"context"
	"time"
)

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

	// QueryLibrary returns a cursor-paginated, filtered, sorted page of works
	// (backend-library-api.md FR-1 through FR-4, FR-9).
	QueryLibrary(ctx context.Context, q LibraryQuery) (*LibraryPage, error)

	// FindWorkDetail returns one Work's detail including owned editions and
	// collection memberships (backend-library-api.md FR-5).
	FindWorkDetail(ctx context.Context, id WorkID) (*WorkDetail, error)
}

type AuthorRepository interface {
	FindByID(ctx context.Context, id AuthorID) (*Author, error)
	Save(ctx context.Context, a *Author) error
}

// EditionRepository. FindByID and FindByWork were phase 02's own minimal
// slice (E4) — FindByID backs LibraryService's Edition-existence check
// (ADR 0020's own named example); FindByWork is what IsInLibrary needs
// to walk a merge group's Editions (domain-library.md FR-2's
// 2026-08-20 amendment, review 0048 finding 3). Save was added by T24
// (tasks/plan-t24-repositories.md, T24-D2) — no phase 02 domain service
// needed it, but real persistence does, and the interface belongs in
// this package regardless of who's about to implement it.
type EditionRepository interface {
	FindByID(ctx context.Context, id EditionID) (*Edition, error)
	FindByWork(ctx context.Context, workID WorkID) ([]*Edition, error)
	Save(ctx context.Context, e *Edition) error
}

// LibraryEntryRepository backs LibraryService's at-most-one-per-Edition
// check (FR-7) and IsInLibrary's existence check.
type LibraryEntryRepository interface {
	// FindByEdition returns a *Error with category NotFound when no
	// entry exists for editionID — never a sentinel error value.
	FindByEdition(ctx context.Context, editionID EditionID) (*LibraryEntry, error)
	Save(ctx context.Context, e *LibraryEntry) error
	DeleteByEdition(ctx context.Context, editionID EditionID) error
}

// CollectionRepository backs CollectionService's Create/Delete and its
// member-mutation round-trips.
type CollectionRepository interface {
	FindByID(ctx context.Context, id CollectionID) (*Collection, error)
	Save(ctx context.Context, c *Collection) error
	Delete(ctx context.Context, id CollectionID) error

	// FindAll returns every collection with its work count (backend-library-api.md FR-6).
	FindAll(ctx context.Context) ([]*CollectionSummary, error)

	// FindDetail returns one collection and its member Works (backend-library-api.md FR-6).
	FindDetail(ctx context.Context, id CollectionID) (*CollectionDetail, error)

	// AddMember adds a Work to a Collection idempotently (backend-library-api.md FR-7).
	AddMember(ctx context.Context, collectionID CollectionID, workID WorkID, addedAt time.Time) error

	// RemoveMember removes a Work's membership from a Collection (backend-library-api.md FR-7).
	RemoveMember(ctx context.Context, collectionID CollectionID, workID WorkID) error

	// Rename renames a Collection (backend-library-api.md FR-6).
	Rename(ctx context.Context, id CollectionID, name string) error
}

// SourceRepository and SourceOfferingRepository back SourceRemovalService
// (domain-source.md FR-6). Save on both was added by T24 (T24-D2) — phase
// 02 never needed to persist a newly-constructed Source or SourceOffering,
// only to read and remove them.
type SourceRepository interface {
	FindByID(ctx context.Context, id SourceID) (*Source, error)
	Save(ctx context.Context, s *Source) error
	Delete(ctx context.Context, id SourceID) error
}

type SourceOfferingRepository interface {
	FindByID(ctx context.Context, id SourceOfferingID) (*SourceOffering, error)
	FindBySource(ctx context.Context, sourceID SourceID) ([]*SourceOffering, error)
	// FindByEdition returns every SourceOffering for editionID, most
	// recently observed first — the fallback order the reader's content
	// path tries sources in (backend-reader-content.md FR-2). An empty
	// slice (never a NotFound error) when the Edition has no offerings.
	FindByEdition(ctx context.Context, editionID EditionID) ([]*SourceOffering, error)
	Save(ctx context.Context, o *SourceOffering) error
	Delete(ctx context.Context, id SourceOfferingID) error
}

// ReadingProgressRepository, BookmarkRepository, HighlightRepository, and
// ReadingPreferencesRepository are new interfaces added by T24 (T24-D2) —
// domain-reading.md's four persisted aggregates had no repository
// interface at all before this, since P20 (tasks/plan-phase02-domain.md)
// only needed EditionRepository, nothing needed to read or write these
// four directly.
type ReadingProgressRepository interface {
	// FindByWork returns a *Error with category NotFound when no
	// ReadingProgress exists for workID yet — FR-1's singleton-per-Work
	// invariant means this is the only lookup shape this type needs.
	FindByWork(ctx context.Context, workID WorkID) (*ReadingProgress, error)
	Save(ctx context.Context, p *ReadingProgress) error
}

type BookmarkRepository interface {
	FindByID(ctx context.Context, id BookmarkID) (*Bookmark, error)
	FindByEdition(ctx context.Context, editionID EditionID) ([]*Bookmark, error)
	Save(ctx context.Context, b *Bookmark) error
	Delete(ctx context.Context, id BookmarkID) error
}

type HighlightRepository interface {
	FindByID(ctx context.Context, id HighlightID) (*Highlight, error)
	FindByEdition(ctx context.Context, editionID EditionID) ([]*Highlight, error)
	Save(ctx context.Context, h *Highlight) error
	Delete(ctx context.Context, id HighlightID) error
}

type ReadingPreferencesRepository interface {
	// FindByDevice returns a *Error with category NotFound when no
	// ReadingPreferences exists for deviceID yet — FR-5's "a new
	// DeviceID's first ReadingPreferences MUST start from system
	// defaults" is the caller's job (construct via NewReadingPreferences
	// on a NotFound), not this repository's.
	FindByDevice(ctx context.Context, deviceID DeviceID) (*ReadingPreferences, error)
	Save(ctx context.Context, p *ReadingPreferences) error
}
