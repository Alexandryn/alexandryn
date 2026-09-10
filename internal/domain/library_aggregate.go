package domain

import (
	"strings"
	"time"
)

// DefaultLibraryID is the well-known identifier for the auto-created system library (ADR 0026).
const DefaultLibraryID = LibraryID("00000000-0000-0000-0000-000000000001")

// Library is the top-level aggregate representing a distinct library namespace (ADR 0026, backend-library-namespaces.md).
type Library struct {
	id                 LibraryID
	name               string
	description        string
	allowReaderUploads bool
	createdAt          time.Time
	updatedAt          time.Time
}

func NewLibrary(id LibraryID, name, description string, allowReaderUploads bool, createdAt, updatedAt time.Time) (*Library, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "library ID cannot be empty"}
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return nil, &Error{Category: InvalidInput, Message: "library name must be between 1 and 100 characters"}
	}
	if len(description) > 2000 {
		return nil, &Error{Category: InvalidInput, Message: "library description cannot exceed 2000 characters"}
	}

	return &Library{
		id:                 id,
		name:               name,
		description:        description,
		allowReaderUploads: allowReaderUploads,
		createdAt:          createdAt,
		updatedAt:          updatedAt,
	}, nil
}

func (l *Library) ID() LibraryID            { return l.id }
func (l *Library) Name() string             { return l.name }
func (l *Library) Description() string      { return l.description }
func (l *Library) AllowReaderUploads() bool { return l.allowReaderUploads }
func (l *Library) CreatedAt() time.Time     { return l.createdAt }
func (l *Library) UpdatedAt() time.Time     { return l.updatedAt }

func (l *Library) Rename(newName string, now time.Time) error {
	newName = strings.TrimSpace(newName)
	if newName == "" || len(newName) > 100 {
		return &Error{Category: InvalidInput, Message: "library name must be between 1 and 100 characters"}
	}
	l.name = newName
	l.updatedAt = now
	return nil
}

func (l *Library) SetDescription(desc string, now time.Time) error {
	if len(desc) > 2000 {
		return &Error{Category: InvalidInput, Message: "library description cannot exceed 2000 characters"}
	}
	l.description = desc
	l.updatedAt = now
	return nil
}

func (l *Library) SetAllowReaderUploads(allow bool, now time.Time) {
	l.allowReaderUploads = allow
	l.updatedAt = now
}

// LibraryMembership represents a user's role and membership in a specific library namespace.
type LibraryMembership struct {
	id        LibraryMembershipID
	libraryID LibraryID
	userID    UserID
	role      Role
	createdAt time.Time
}

func NewLibraryMembership(id LibraryMembershipID, libraryID LibraryID, userID UserID, role Role, createdAt time.Time) (*LibraryMembership, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "membership ID cannot be empty"}
	}
	if strings.TrimSpace(string(libraryID)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "library ID cannot be empty"}
	}
	if strings.TrimSpace(string(userID)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "user ID cannot be empty"}
	}
	if !role.IsValid() {
		return nil, &Error{Category: InvalidInput, Message: "invalid membership role"}
	}

	return &LibraryMembership{
		id:        id,
		libraryID: libraryID,
		userID:    userID,
		role:      role,
		createdAt: createdAt,
	}, nil
}

func (m *LibraryMembership) ID() LibraryMembershipID { return m.id }
func (m *LibraryMembership) LibraryID() LibraryID    { return m.libraryID }
func (m *LibraryMembership) UserID() UserID          { return m.userID }
func (m *LibraryMembership) Role() Role              { return m.role }
func (m *LibraryMembership) CreatedAt() time.Time    { return m.createdAt }

func (m *LibraryMembership) SetRole(r Role) error {
	if !r.IsValid() {
		return &Error{Category: InvalidInput, Message: "invalid membership role"}
	}
	m.role = r
	return nil
}

// LibraryInvitation represents an invitation for an email address to join a library.
type LibraryInvitation struct {
	id        LibraryInvitationID
	libraryID LibraryID
	email     string
	role      Role
	tokenHash string
	createdBy UserID
	expiresAt time.Time
	usedAt    *time.Time
	createdAt time.Time
}

func NewLibraryInvitation(id LibraryInvitationID, libraryID LibraryID, email string, role Role, tokenHash string, createdBy UserID, expiresAt, createdAt time.Time) (*LibraryInvitation, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "invitation ID cannot be empty"}
	}
	if strings.TrimSpace(string(libraryID)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "library ID cannot be empty"}
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if len(email) > 255 || !validEmailRegex.MatchString(email) {
		return nil, &Error{Category: InvalidInput, Message: "invalid email format"}
	}
	if !role.IsValid() {
		return nil, &Error{Category: InvalidInput, Message: "invalid invitation role"}
	}
	if strings.TrimSpace(tokenHash) == "" {
		return nil, &Error{Category: InvalidInput, Message: "token hash cannot be empty"}
	}
	if strings.TrimSpace(string(createdBy)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "created by user ID cannot be empty"}
	}

	return &LibraryInvitation{
		id:        id,
		libraryID: libraryID,
		email:     email,
		role:      role,
		tokenHash: tokenHash,
		createdBy: createdBy,
		expiresAt: expiresAt,
		createdAt: createdAt,
	}, nil
}

func RehydrateLibraryInvitation(id LibraryInvitationID, libraryID LibraryID, email string, role Role, tokenHash string, createdBy UserID, expiresAt time.Time, usedAt *time.Time, createdAt time.Time) *LibraryInvitation {
	return &LibraryInvitation{
		id:        id,
		libraryID: libraryID,
		email:     email,
		role:      role,
		tokenHash: tokenHash,
		createdBy: createdBy,
		expiresAt: expiresAt,
		usedAt:    usedAt,
		createdAt: createdAt,
	}
}

func (inv *LibraryInvitation) ID() LibraryInvitationID { return inv.id }
func (inv *LibraryInvitation) LibraryID() LibraryID    { return inv.libraryID }
func (inv *LibraryInvitation) Email() string           { return inv.email }
func (inv *LibraryInvitation) Role() Role              { return inv.role }
func (inv *LibraryInvitation) TokenHash() string       { return inv.tokenHash }
func (inv *LibraryInvitation) CreatedBy() UserID       { return inv.createdBy }
func (inv *LibraryInvitation) ExpiresAt() time.Time    { return inv.expiresAt }
func (inv *LibraryInvitation) UsedAt() *time.Time      { return inv.usedAt }
func (inv *LibraryInvitation) CreatedAt() time.Time    { return inv.createdAt }

func (inv *LibraryInvitation) IsValid(now time.Time) bool {
	return inv.usedAt == nil && now.Before(inv.expiresAt)
}

func (inv *LibraryInvitation) MarkUsed(now time.Time) {
	inv.usedAt = &now
}
