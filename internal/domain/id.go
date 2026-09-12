package domain

// Distinct named ID types per aggregate prevent accidental substitution
// (e.g. passing a WorkID where an AuthorID is expected fails to compile)
// at zero runtime cost.
type (
	WorkID            string
	EditionID         string
	AuthorID          string
	LibraryEntryID    string
	CollectionID      string
	SourceID          string
	SourceOfferingID  string
	ReadingProgressID string
	BookmarkID        string
	HighlightID       string

	// Authentication, RBAC, and multi-library namespacing IDs
	UserID              string
	LibraryID           string
	LibraryMembershipID string
	LibraryInvitationID string
	RefreshTokenID      string
	PasswordResetID     string

	// DeviceID identifies a registered device.
	DeviceID string

	// PairingSessionID identifies an active pairing session.
	PairingSessionID string
)

// IDGenerator produces a new, unique identifier value at aggregate
// construction time ("generated at creation, never derived from external data").
// The single string-returning method is deliberately untyped per-aggregate —
// callers convert the result to the ID type they need (e.g., WorkID(gen.NewID()))
// so one interface and one injected implementation serves every aggregate.
type IDGenerator interface {
	NewID() string
}
