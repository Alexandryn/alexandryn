package domain

import "time"

// Event is the sealed base interface every domain event satisfies. The
// unexported isDomainEvent method seals it to this package so no type
// outside internal/domain can implement Event, ensuring the
// PublicEvent and SensitiveEvent split is exhaustive.
type Event interface {
	Type() string
	AggregateID() string
	OccurredAt() time.Time
	isDomainEvent()
}

// PublicEvent and SensitiveEvent are two mutually exclusive markers that
// concrete event types implement. Sensitivity is carried by the type itself
// rather than a runtime method, allowing compile-time enforcement so logging
// sinks declared over PublicEvent cannot accept SensitiveEvent values.
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

// --- PublicEvent catalog (13) ---

// WorkCreated, EditionCreated, and AuthorCreated are emitted when a record
// enters the catalog independently of library acquisition (e.g. metadata discovery).
// WorkImported and EditionImported are used for the acquisition import path.
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

// WorkMerged, WorkMergeUndone, AuthorMerged, and AuthorMergeUndone represent
// merge operations and their corresponding reversals.
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

// WorkContainsAdded and WorkContainsRemoved represent containment relationships
// between works, subject to cycle prevention.
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

// SourceCreated, SourceRemoved, SourceOfferingObserved, and SourceOfferingRemoved
// track the lifecycle and offerings of external sources, which carry no private user data.
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

// SourceOfferingRemoved is emitted when an offering is removed, including
// when cascading from source removal.
type SourceOfferingRemoved struct {
	baseEvent
	publicMarker
}

func (SourceOfferingRemoved) Type() string { return "SourceOfferingRemoved" }

func NewSourceOfferingRemoved(aggregateID string, occurredAt time.Time) SourceOfferingRemoved {
	return SourceOfferingRemoved{baseEvent: newBaseEvent(aggregateID, occurredAt)}
}

// --- SensitiveEvent catalog (10) ---

// WorkImported and EditionImported represent record creation during file acquisition,
// which discloses library contents and is classified as sensitive.
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

// LibraryEntryAdded and LibraryEntryRemoved track book possession in a user's
// library and are classified as sensitive under the privacy policy.
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

// CollectionCreated, CollectionMemberAdded, and CollectionMemberRemoved track
// user-curated collections and reading lists, which are classified as sensitive.
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

// ReadingProgressUpdated, BookmarkCreated, and HighlightCreated track user reading
// activity, progress, and annotations, strictly classified as sensitive under
// the privacy policy.
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
