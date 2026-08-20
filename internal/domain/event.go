package domain

import "time"

// Event is the sealed base every domain event satisfies (domain-events.md
// FR-1). The unexported isDomainEvent method seals it to this package —
// no type outside internal/domain can implement Event, which is what
// makes the PublicEvent/SensitiveEvent split below exhaustive: no event
// can exist that is neither public nor sensitive.
type Event interface {
	Type() string
	AggregateID() string
	OccurredAt() time.Time
	isDomainEvent()
}

// PublicEvent and SensitiveEvent are the two mutually exclusive markers
// every concrete event type below implements exactly one of (FR-1).
// Sensitivity is carried by the type, never computed from an event's own
// data or a Sensitive() bool method — Go cannot dispatch on a method's
// return value, so a bool-returning classification cannot stop a logging
// sink from accepting a stream carrying sensitive events (review 0048
// finding 4). A sink declared over PublicEvent cannot be handed a
// SensitiveEvent — that's a compile error, not a runtime check.
type PublicEvent interface {
	Event
	isPublicEvent()
}

type SensitiveEvent interface {
	Event
	isSensitiveEvent()
}

type baseEvent struct {
	aggregateID string
	occurredAt  time.Time
}

func (e baseEvent) AggregateID() string   { return e.aggregateID }
func (e baseEvent) OccurredAt() time.Time { return e.occurredAt }
func (baseEvent) isDomainEvent()          {}

func newBaseEvent(aggregateID string, occurredAt time.Time) baseEvent {
	return baseEvent{aggregateID: aggregateID, occurredAt: occurredAt}
}

type publicMarker struct{}

func (publicMarker) isPublicEvent() {}

type sensitiveMarker struct{}

func (sensitiveMarker) isSensitiveEvent() {}

// --- PublicEvent catalog (13) — domain-bibliographic.md, domain-source.md ---

// WorkCreated/EditionCreated/AuthorCreated are the catalog-entry path
// (FR-2's acquisition split): emitted when a record enters the catalog
// without entering anyone's possession (e.g. Discover browsing). The
// phase-10 import path emits WorkImported/EditionImported instead.
type WorkCreated struct {
	baseEvent
	publicMarker
}

func (WorkCreated) Type() string { return "WorkCreated" }

func NewWorkCreated(aggregateID string, occurredAt time.Time) WorkCreated {
	return WorkCreated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type EditionCreated struct {
	baseEvent
	publicMarker
}

func (EditionCreated) Type() string { return "EditionCreated" }

func NewEditionCreated(aggregateID string, occurredAt time.Time) EditionCreated {
	return EditionCreated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type AuthorCreated struct {
	baseEvent
	publicMarker
}

func (AuthorCreated) Type() string { return "AuthorCreated" }

func NewAuthorCreated(aggregateID string, occurredAt time.Time) AuthorCreated {
	return AuthorCreated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// WorkMerged/WorkMergeUndone/AuthorMerged/AuthorMergeUndone: the merge
// mechanism's own events (domain-bibliographic.md FR-4/FR-7), reversible
// pairs — an undo event exists because the merge itself does.
type WorkMerged struct {
	baseEvent
	publicMarker
}

func (WorkMerged) Type() string { return "WorkMerged" }

func NewWorkMerged(aggregateID string, occurredAt time.Time) WorkMerged {
	return WorkMerged{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type WorkMergeUndone struct {
	baseEvent
	publicMarker
}

func (WorkMergeUndone) Type() string { return "WorkMergeUndone" }

func NewWorkMergeUndone(aggregateID string, occurredAt time.Time) WorkMergeUndone {
	return WorkMergeUndone{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type AuthorMerged struct {
	baseEvent
	publicMarker
}

func (AuthorMerged) Type() string { return "AuthorMerged" }

func NewAuthorMerged(aggregateID string, occurredAt time.Time) AuthorMerged {
	return AuthorMerged{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type AuthorMergeUndone struct {
	baseEvent
	publicMarker
}

func (AuthorMergeUndone) Type() string { return "AuthorMergeUndone" }

func NewAuthorMergeUndone(aggregateID string, occurredAt time.Time) AuthorMergeUndone {
	return AuthorMergeUndone{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// WorkContainsAdded/WorkContainsRemoved: the omnibus containment mutation
// (domain-bibliographic.md FR-9), cycle-checked the same way merges are.
type WorkContainsAdded struct {
	baseEvent
	publicMarker
}

func (WorkContainsAdded) Type() string { return "WorkContainsAdded" }

func NewWorkContainsAdded(aggregateID string, occurredAt time.Time) WorkContainsAdded {
	return WorkContainsAdded{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type WorkContainsRemoved struct {
	baseEvent
	publicMarker
}

func (WorkContainsRemoved) Type() string { return "WorkContainsRemoved" }

func NewWorkContainsRemoved(aggregateID string, occurredAt time.Time) WorkContainsRemoved {
	return WorkContainsRemoved{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// SourceCreated/SourceRemoved/SourceOfferingObserved/SourceOfferingRemoved
// (domain-source.md): a Source's own lifecycle and what it claims to
// offer are never sensitive — they say nothing about what the user owns.
type SourceCreated struct {
	baseEvent
	publicMarker
}

func (SourceCreated) Type() string { return "SourceCreated" }

func NewSourceCreated(aggregateID string, occurredAt time.Time) SourceCreated {
	return SourceCreated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type SourceRemoved struct {
	baseEvent
	publicMarker
}

func (SourceRemoved) Type() string { return "SourceRemoved" }

func NewSourceRemoved(aggregateID string, occurredAt time.Time) SourceRemoved {
	return SourceRemoved{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type SourceOfferingObserved struct {
	baseEvent
	publicMarker
}

func (SourceOfferingObserved) Type() string { return "SourceOfferingObserved" }

func NewSourceOfferingObserved(aggregateID string, occurredAt time.Time) SourceOfferingObserved {
	return SourceOfferingObserved{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// SourceOfferingRemoved is domain-source.md FR-6's cascade-delete
// counterpart — emitted once per SourceOffering removed when a Source is
// removed, not once per Source.
type SourceOfferingRemoved struct {
	baseEvent
	publicMarker
}

func (SourceOfferingRemoved) Type() string { return "SourceOfferingRemoved" }

func NewSourceOfferingRemoved(aggregateID string, occurredAt time.Time) SourceOfferingRemoved {
	return SourceOfferingRemoved{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// --- SensitiveEvent catalog (10) — domain-bibliographic.md, domain-library.md, domain-reading.md ---

// WorkImported/EditionImported are FR-2's acquisition-path counterparts
// to WorkCreated/EditionCreated: the same record creation, but as part of
// acquiring a file — which is exactly what LibraryEntryAdded is protected
// for, so this path discloses the same fact and must be classified the
// same way.
type WorkImported struct {
	baseEvent
	sensitiveMarker
}

func (WorkImported) Type() string { return "WorkImported" }

func NewWorkImported(aggregateID string, occurredAt time.Time) WorkImported {
	return WorkImported{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type EditionImported struct {
	baseEvent
	sensitiveMarker
}

func (EditionImported) Type() string { return "EditionImported" }

func NewEditionImported(aggregateID string, occurredAt time.Time) EditionImported {
	return EditionImported{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// LibraryEntryAdded/LibraryEntryRemoved (domain-library.md FR-1/FR-6):
// "owns" is "reads" for constitution §8's purposes — a list of what
// someone owns is revealing, treated as sensitive by the more protective
// reading.
type LibraryEntryAdded struct {
	baseEvent
	sensitiveMarker
}

func (LibraryEntryAdded) Type() string { return "LibraryEntryAdded" }

func NewLibraryEntryAdded(aggregateID string, occurredAt time.Time) LibraryEntryAdded {
	return LibraryEntryAdded{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type LibraryEntryRemoved struct {
	baseEvent
	sensitiveMarker
}

func (LibraryEntryRemoved) Type() string { return "LibraryEntryRemoved" }

func NewLibraryEntryRemoved(aggregateID string, occurredAt time.Time) LibraryEntryRemoved {
	return LibraryEntryRemoved{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// CollectionCreated/CollectionMemberAdded/CollectionMemberRemoved
// (domain-library.md FR-4): a curated "want to read" list reveals
// book-level interest as directly as ownership does. CollectionCreated
// is sensitive even with zero members — the collection's own
// user-chosen name can itself disclose sensitive interest.
type CollectionCreated struct {
	baseEvent
	sensitiveMarker
}

func (CollectionCreated) Type() string { return "CollectionCreated" }

func NewCollectionCreated(aggregateID string, occurredAt time.Time) CollectionCreated {
	return CollectionCreated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type CollectionMemberAdded struct {
	baseEvent
	sensitiveMarker
}

func (CollectionMemberAdded) Type() string { return "CollectionMemberAdded" }

func NewCollectionMemberAdded(aggregateID string, occurredAt time.Time) CollectionMemberAdded {
	return CollectionMemberAdded{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type CollectionMemberRemoved struct {
	baseEvent
	sensitiveMarker
}

func (CollectionMemberRemoved) Type() string { return "CollectionMemberRemoved" }

func NewCollectionMemberRemoved(aggregateID string, occurredAt time.Time) CollectionMemberRemoved {
	return CollectionMemberRemoved{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// ReadingProgressUpdated/BookmarkCreated/HighlightCreated
// (domain-reading.md): constitution §8's direct case — what someone
// reads, never logged, including from their own log files.
// ReadingProgressUpdated's real emission site is ReconcileProgress
// (domain-reading.md FR-6/FR-7), which this plan does not implement —
// separately blocked, unrelated defect — so this type exists with no
// real emitter yet, named in the plan rather than silently assumed
// covered.
type ReadingProgressUpdated struct {
	baseEvent
	sensitiveMarker
}

func (ReadingProgressUpdated) Type() string { return "ReadingProgressUpdated" }

func NewReadingProgressUpdated(aggregateID string, occurredAt time.Time) ReadingProgressUpdated {
	return ReadingProgressUpdated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type BookmarkCreated struct {
	baseEvent
	sensitiveMarker
}

func (BookmarkCreated) Type() string { return "BookmarkCreated" }

func NewBookmarkCreated(aggregateID string, occurredAt time.Time) BookmarkCreated {
	return BookmarkCreated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

type HighlightCreated struct {
	baseEvent
	sensitiveMarker
}

func (HighlightCreated) Type() string { return "HighlightCreated" }

func NewHighlightCreated(aggregateID string, occurredAt time.Time) HighlightCreated {
	return HighlightCreated{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}
