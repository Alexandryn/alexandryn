package domain

import (
	"context"
	"time"
)

// WorkRepository and AuthorRepository are declared by internal/domain
// and satisfied by persistence implementations — a domain service performs IO
// through an interface the domain itself owns, never a persistence-specific type.
//
// Both are minimal interfaces containing what domain services need to
// enforce their own invariants.
//
// FindByID returns a *Error with category NotFound when id doesn't
// exist — callers rely on CategoryOf to distinguish "not found" from any
// other failure, never a sentinel error value.
type WorkRepository interface {
	FindByID(ctx context.Context, id WorkID) (*Work, error)

	// FindMergedInto returns every Work whose MergedInto directly equals
	// canonical (one hop, not transitive) — the primitive
	// WorkContainmentService needs to compute the merge-resolved
	// containment union across that Work and every Work merged into it,
	// transitively.
	FindMergedInto(ctx context.Context, canonical WorkID) ([]*Work, error)

	Save(ctx context.Context, w *Work) error

	// QueryLibrary returns a cursor-paginated, filtered, sorted page of works.
	QueryLibrary(ctx context.Context, q LibraryQuery) (*LibraryPage, error)

	// FindWorkDetail returns one Work's detail including owned editions and
	// collection memberships, scoped to libraryID: only that library's
	// editions/memberships, and NotFound when the work is in no form present
	// in that library.
	FindWorkDetail(ctx context.Context, id WorkID, libraryID LibraryID) (*WorkDetail, error)
}

type AuthorRepository interface {
	FindByID(ctx context.Context, id AuthorID) (*Author, error)
	Save(ctx context.Context, a *Author) error
}

// EditionRepository provides access to persisted Editions. FindByID backs
// LibraryService's Edition-existence check; FindByWork is what IsInLibrary
// needs to walk a merge group's Editions.
type EditionRepository interface {
	FindByID(ctx context.Context, id EditionID) (*Edition, error)
	FindByWork(ctx context.Context, workID WorkID) ([]*Edition, error)
	Save(ctx context.Context, e *Edition) error
}

// LibraryEntryRepository backs LibraryService's at-most-one-per-Edition
// check and IsInLibrary's existence check.
type LibraryEntryRepository interface {
	// FindByEdition returns a *Error with category NotFound when no
	// entry exists for editionID — never a sentinel error value.
	FindByEdition(ctx context.Context, editionID EditionID) (*LibraryEntry, error)
	// EditionInLibrary reports whether editionID is owned in libraryID.
	// The reader-content path gates on this so a member of library X
	// cannot stream the bytes of an edition owned only in library Y.
	EditionInLibrary(ctx context.Context, editionID EditionID, libraryID LibraryID) (bool, error)
	// WorkInLibrary reports whether libraryID owns at least one edition of
	// workID — the Work-keyed equivalent of EditionInLibrary, for gating
	// progress writes.
	WorkInLibrary(ctx context.Context, workID WorkID, libraryID LibraryID) (bool, error)
	Save(ctx context.Context, e *LibraryEntry) error
	DeleteByEdition(ctx context.Context, editionID EditionID) error
}

// CollectionRepository backs CollectionService's Create/Delete and its
// member-mutation round-trips. Every method is scoped to one library:
// collections belong to the Alexandryn library they were created in, and
// a caller in another library must not see or mutate them (constitution §3/§6).
// A collection id that exists in a different library is reported as
// NotFound — no cross-library existence oracle.
type CollectionRepository interface {
	FindByID(ctx context.Context, libraryID LibraryID, id CollectionID) (*Collection, error)
	Save(ctx context.Context, libraryID LibraryID, c *Collection) error
	Delete(ctx context.Context, libraryID LibraryID, id CollectionID) error

	// FindAll returns every collection with its work count.
	FindAll(ctx context.Context, libraryID LibraryID) ([]*CollectionSummary, error)

	// FindDetail returns one collection and its member Works.
	FindDetail(ctx context.Context, libraryID LibraryID, id CollectionID) (*CollectionDetail, error)

	// AddMember adds a Work to a Collection idempotently.
	AddMember(ctx context.Context, libraryID LibraryID, collectionID CollectionID, workID WorkID, addedAt time.Time) error

	// RemoveMember removes a Work's membership from a Collection.
	RemoveMember(ctx context.Context, libraryID LibraryID, collectionID CollectionID, workID WorkID) error

	// Rename renames a Collection.
	Rename(ctx context.Context, libraryID LibraryID, id CollectionID, name string) error
}

// SourceRepository and SourceOfferingRepository back SourceRemovalService.
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
	// path tries sources in. An empty slice (never a NotFound error)
	// when the Edition has no offerings.
	FindByEdition(ctx context.Context, editionID EditionID) ([]*SourceOffering, error)
	Save(ctx context.Context, o *SourceOffering) error
	Delete(ctx context.Context, id SourceOfferingID) error
}

// ReadingProgressRepository, BookmarkRepository, HighlightRepository, and
// ReadingPreferencesRepository manage reading state aggregates.
type ReadingProgressRepository interface {
	// FindByWork returns a *Error with category NotFound when no
	// ReadingProgress exists for workID yet — singleton-per-Work
	// invariant means this is the only lookup shape this type needs.
	FindByWork(ctx context.Context, workID WorkID) (*ReadingProgress, error)
	// FindByWorkForUpdate is FindByWork with a row lock (SELECT ... FOR
	// UPDATE), for the reconcile-and-persist transaction — the read and
	// the write must be atomic or two concurrent reports lose an update.
	// NotFound when none exists yet (the caller then inserts the first
	// canonical row, racing on the work_id UNIQUE constraint).
	FindByWorkForUpdate(ctx context.Context, workID WorkID) (*ReadingProgress, error)
	// FindByWorkAndUser supports user- and library-scoped reading progress.
	FindByWorkAndUser(ctx context.Context, userID UserID, libraryID LibraryID, workID WorkID) (*ReadingProgress, error)
	FindByWorkAndUserForUpdate(ctx context.Context, userID UserID, libraryID LibraryID, workID WorkID) (*ReadingProgress, error)
	Save(ctx context.Context, p *ReadingProgress) error
	SaveForUser(ctx context.Context, userID UserID, libraryID LibraryID, p *ReadingProgress) error
}

type BookmarkRepository interface {
	FindByID(ctx context.Context, id BookmarkID) (*Bookmark, error)
	// FindByIDAndUser returns a *Error with category NotFound when the
	// bookmark does not exist OR belongs to another user — a caller must
	// not be able to tell the two apart.
	FindByIDAndUser(ctx context.Context, userID UserID, id BookmarkID) (*Bookmark, error)
	FindByEdition(ctx context.Context, editionID EditionID) ([]*Bookmark, error)
	FindByEditionAndUser(ctx context.Context, userID UserID, libraryID LibraryID, editionID EditionID) ([]*Bookmark, error)
	Save(ctx context.Context, b *Bookmark) error
	SaveForUser(ctx context.Context, userID UserID, libraryID LibraryID, b *Bookmark) error
	Delete(ctx context.Context, id BookmarkID) error
	// DeleteAndUser deletes only when the row belongs to userID; a foreign
	// or missing id is a NotFound, not a silent success.
	DeleteAndUser(ctx context.Context, userID UserID, id BookmarkID) error
}

type HighlightRepository interface {
	FindByID(ctx context.Context, id HighlightID) (*Highlight, error)
	FindByIDAndUser(ctx context.Context, userID UserID, id HighlightID) (*Highlight, error)
	FindByEdition(ctx context.Context, editionID EditionID) ([]*Highlight, error)
	FindByEditionAndUser(ctx context.Context, userID UserID, libraryID LibraryID, editionID EditionID) ([]*Highlight, error)
	Save(ctx context.Context, h *Highlight) error
	SaveForUser(ctx context.Context, userID UserID, libraryID LibraryID, h *Highlight) error
	// UpdateNoteCategoryAndUser updates only the note and category of a
	// highlight the user owns, leaving edition_id and library_id
	// untouched — a PATCH must not relocate the highlight into whatever
	// library the request's active-library header names.
	// A foreign or missing id is a NotFound.
	UpdateNoteCategoryAndUser(ctx context.Context, userID UserID, id HighlightID, note, category string) error
	Delete(ctx context.Context, id HighlightID) error
	DeleteAndUser(ctx context.Context, userID UserID, id HighlightID) error
}

type ReadingPreferencesRepository interface {
	// FindByDevice returns a *Error with category NotFound when no
	// ReadingPreferences exists for deviceID yet. Initializing default
	// preferences is the caller's responsibility via NewReadingPreferences
	// on a NotFound.
	FindByDevice(ctx context.Context, deviceID DeviceID) (*ReadingPreferences, error)
	FindByUserAndDevice(ctx context.Context, userID UserID, deviceID DeviceID) (*ReadingPreferences, error)
	Save(ctx context.Context, p *ReadingPreferences) error
	SaveForUser(ctx context.Context, userID UserID, p *ReadingPreferences) error
}

// Authentication & Tenancy Repositories

type UserRepository interface {
	FindByID(ctx context.Context, id UserID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	CountUsers(ctx context.Context) (int, error)
	Save(ctx context.Context, u *User) error
}

type CredentialRepository interface {
	FindByUserID(ctx context.Context, userID UserID) (*UserCredentials, error)
	Save(ctx context.Context, creds *UserCredentials) error
}

type RefreshTokenRepository interface {
	FindByTokenHash(ctx context.Context, hash string) (*RefreshToken, error)
	Save(ctx context.Context, rt *RefreshToken) error
	RevokeAllForUser(ctx context.Context, userID UserID, now time.Time) error
}

type MFARepository interface {
	FindByUserID(ctx context.Context, userID UserID) (*TOTPSettings, error)
	Save(ctx context.Context, s *TOTPSettings) error
	Delete(ctx context.Context, userID UserID) error
}

type PasswordResetRepository interface {
	FindByTokenHash(ctx context.Context, hash string) (*PasswordResetToken, error)
	Save(ctx context.Context, prt *PasswordResetToken) error
}

type LibraryRepository interface {
	FindByID(ctx context.Context, id LibraryID) (*Library, error)
	FindAll(ctx context.Context) ([]*Library, error)
	FindByUser(ctx context.Context, userID UserID) ([]*Library, error)
	Save(ctx context.Context, l *Library) error
	Delete(ctx context.Context, id LibraryID) error
}

type LibraryMembershipRepository interface {
	FindMembership(ctx context.Context, libraryID LibraryID, userID UserID) (*LibraryMembership, error)
	FindByLibrary(ctx context.Context, libraryID LibraryID) ([]*LibraryMembership, error)
	FindByUser(ctx context.Context, userID UserID) ([]*LibraryMembership, error)
	Save(ctx context.Context, m *LibraryMembership) error
	Delete(ctx context.Context, libraryID LibraryID, userID UserID) error
}

type LibraryInvitationRepository interface {
	FindByID(ctx context.Context, id LibraryInvitationID) (*LibraryInvitation, error)
	FindByTokenHash(ctx context.Context, hash string) (*LibraryInvitation, error)
	FindByLibrary(ctx context.Context, libraryID LibraryID) ([]*LibraryInvitation, error)
	Save(ctx context.Context, inv *LibraryInvitation) error
	Delete(ctx context.Context, id LibraryInvitationID) error
}

// Network Access & Device Pairing Repositories

type PairingSessionRepository interface {
	FindByID(ctx context.Context, id PairingSessionID) (*PairingSession, error)
	FindByCodeIndex(ctx context.Context, codeIndex []byte) (*PairingSession, error)
	FindPendingByCodeIndexForUpdate(ctx context.Context, codeIndex []byte, now time.Time) (*PairingSession, error)
	Save(ctx context.Context, s *PairingSession) error
	SaveWithInitiatorIP(ctx context.Context, s *PairingSession, ip string) error
	Delete(ctx context.Context, id PairingSessionID) error
}

type PairedDeviceRepository interface {
	FindByID(ctx context.Context, id DeviceID) (*PairedDevice, error)
	FindByOwner(ctx context.Context, owner UserID) ([]*PairedDevice, error)
	FindByPairingSessionID(ctx context.Context, sessionID PairingSessionID) (*PairedDevice, error)
	InsertProvisional(ctx context.Context, id DeviceID, label string, deviceClass DeviceClass, enrolledVia EnrolledVia, pairingSessionID PairingSessionID, now time.Time) error
	AssignOwnerByPairingSession(ctx context.Context, sessionID PairingSessionID, owner UserID) error
	Save(ctx context.Context, d *PairedDevice) error
	Revoke(ctx context.Context, id DeviceID, now time.Time) error
	RevokeByPairingSessionID(ctx context.Context, sessionID PairingSessionID, now time.Time) error
	AdvanceCursor(ctx context.Context, id DeviceID, newCursor int64, now time.Time) error
	UpdateLastSeen(ctx context.Context, id DeviceID, now time.Time) error
}

type NetworkSettingsRepository interface {
	Get(ctx context.Context) (*NetworkSettings, error)
	Upsert(ctx context.Context, s *NetworkSettings) error
}

type EnrolmentGrantJTIRepository interface {
	// Claim atomically records jti as spent and reports whether this call
	// was the one that spent it. A second call with the same jti returns
	// (false, nil). This is the single-use gate for an enrolment grant:
	// a separate Exists-then-Record pair is a TOCTOU race under concurrent
	// logins with the same grant.
	Claim(ctx context.Context, jti string, spentAt time.Time) (bool, error)
}

// MFATicketJTIRepository is the single-use gate for an MFA ticket. A
// ticket carries a jti (auth.SignMFATicket); TOTPVerifyHandler claims it
// on presentation so a captured ticket cannot be replayed within its TTL.
type MFATicketJTIRepository interface {
	Claim(ctx context.Context, jti string, spentAt time.Time) (bool, error)
}
