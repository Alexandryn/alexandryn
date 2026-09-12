package domain

import (
	"regexp"
	"strings"
	"time"
)

// Role represents a user's system-wide or per-library role.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleReader Role = "reader"
)

var (
	validUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)
	validEmailRegex    = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)
)

func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleReader:
		return RoleReader, nil
	default:
		return "", &Error{Category: InvalidInput, Message: "invalid role: " + s}
	}
}

func (r Role) IsValid() bool {
	return r == RoleAdmin || r == RoleReader
}

// User represents an authenticated identity in Alexandryn.
type User struct {
	id        UserID
	username  string
	email     string
	role      Role
	createdAt time.Time
	updatedAt time.Time
}

func NewUser(id UserID, username, email string, role Role, createdAt, updatedAt time.Time) (*User, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "user ID cannot be empty"}
	}
	username = strings.TrimSpace(username)
	if !validUsernameRegex.MatchString(username) {
		return nil, &Error{Category: InvalidInput, Message: "username must be 3-50 alphanumeric characters or - _"}
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if len(email) > 255 || !validEmailRegex.MatchString(email) {
		return nil, &Error{Category: InvalidInput, Message: "invalid email address format"}
	}
	if !role.IsValid() {
		return nil, &Error{Category: InvalidInput, Message: "invalid user role"}
	}

	return &User{
		id:        id,
		username:  username,
		email:     email,
		role:      role,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}, nil
}

func (u *User) ID() UserID           { return u.id }
func (u *User) Username() string     { return u.username }
func (u *User) Email() string        { return u.email }
func (u *User) Role() Role           { return u.role }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

func (u *User) SetRole(r Role, now time.Time) error {
	if !r.IsValid() {
		return &Error{Category: InvalidInput, Message: "invalid user role"}
	}
	u.role = r
	u.updatedAt = now
	return nil
}

// UserCredentials stores the Argon2id password hash for a user.
type UserCredentials struct {
	userID       UserID
	passwordHash string
	updatedAt    time.Time
}

func NewUserCredentials(userID UserID, passwordHash string, updatedAt time.Time) (*UserCredentials, error) {
	if strings.TrimSpace(string(userID)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "user ID cannot be empty"}
	}
	if strings.TrimSpace(passwordHash) == "" {
		return nil, &Error{Category: InvalidInput, Message: "password hash cannot be empty"}
	}
	return &UserCredentials{
		userID:       userID,
		passwordHash: passwordHash,
		updatedAt:    updatedAt,
	}, nil
}

func (c *UserCredentials) UserID() UserID       { return c.userID }
func (c *UserCredentials) PasswordHash() string { return c.passwordHash }
func (c *UserCredentials) UpdatedAt() time.Time { return c.updatedAt }

// RefreshToken represents a stateful, rotatable session refresh token.
type RefreshToken struct {
	id        RefreshTokenID
	userID    UserID
	tokenHash string
	expiresAt time.Time
	revokedAt *time.Time
	createdAt time.Time
}

func NewRefreshToken(id RefreshTokenID, userID UserID, tokenHash string, expiresAt, createdAt time.Time) (*RefreshToken, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "refresh token ID cannot be empty"}
	}
	if strings.TrimSpace(string(userID)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "user ID cannot be empty"}
	}
	if strings.TrimSpace(tokenHash) == "" {
		return nil, &Error{Category: InvalidInput, Message: "token hash cannot be empty"}
	}
	return &RefreshToken{
		id:        id,
		userID:    userID,
		tokenHash: tokenHash,
		expiresAt: expiresAt,
		createdAt: createdAt,
	}, nil
}

func RehydrateRefreshToken(id RefreshTokenID, userID UserID, tokenHash string, expiresAt time.Time, revokedAt *time.Time, createdAt time.Time) *RefreshToken {
	return &RefreshToken{
		id:        id,
		userID:    userID,
		tokenHash: tokenHash,
		expiresAt: expiresAt,
		revokedAt: revokedAt,
		createdAt: createdAt,
	}
}

func (r *RefreshToken) ID() RefreshTokenID    { return r.id }
func (r *RefreshToken) UserID() UserID        { return r.userID }
func (r *RefreshToken) TokenHash() string     { return r.tokenHash }
func (r *RefreshToken) ExpiresAt() time.Time  { return r.expiresAt }
func (r *RefreshToken) RevokedAt() *time.Time { return r.revokedAt }
func (r *RefreshToken) CreatedAt() time.Time  { return r.createdAt }

func (r *RefreshToken) IsActive(now time.Time) bool {
	return r.revokedAt == nil && now.Before(r.expiresAt)
}

func (r *RefreshToken) Revoke(now time.Time) {
	r.revokedAt = &now
}

// TOTPSettings records MFA settings and encrypted secrets.
type TOTPSettings struct {
	userID             UserID
	encryptedSecret    []byte
	recoveryCodeHashes []string
	enabled            bool
	confirmedAt        *time.Time
	createdAt          time.Time
}

func NewTOTPSettings(userID UserID, encryptedSecret []byte, recoveryCodeHashes []string, enabled bool, confirmedAt *time.Time, createdAt time.Time) (*TOTPSettings, error) {
	if strings.TrimSpace(string(userID)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "user ID cannot be empty"}
	}
	if len(encryptedSecret) == 0 {
		return nil, &Error{Category: InvalidInput, Message: "encrypted secret cannot be empty"}
	}
	return &TOTPSettings{
		userID:             userID,
		encryptedSecret:    encryptedSecret,
		recoveryCodeHashes: recoveryCodeHashes,
		enabled:            enabled,
		confirmedAt:        confirmedAt,
		createdAt:          createdAt,
	}, nil
}

func (t *TOTPSettings) UserID() UserID               { return t.userID }
func (t *TOTPSettings) EncryptedSecret() []byte      { return t.encryptedSecret }
func (t *TOTPSettings) RecoveryCodeHashes() []string { return t.recoveryCodeHashes }
func (t *TOTPSettings) IsEnabled() bool              { return t.enabled }
func (t *TOTPSettings) ConfirmedAt() *time.Time      { return t.confirmedAt }
func (t *TOTPSettings) CreatedAt() time.Time         { return t.createdAt }

func (t *TOTPSettings) Confirm(now time.Time) {
	t.enabled = true
	t.confirmedAt = &now
}

func (t *TOTPSettings) Disable() {
	t.enabled = false
	t.confirmedAt = nil
}

// PasswordResetToken represents a single-use, time-limited password recovery token.
type PasswordResetToken struct {
	id        PasswordResetID
	userID    UserID
	tokenHash string
	expiresAt time.Time
	usedAt    *time.Time
	createdAt time.Time
}

func NewPasswordResetToken(id PasswordResetID, userID UserID, tokenHash string, expiresAt, createdAt time.Time) (*PasswordResetToken, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "password reset token ID cannot be empty"}
	}
	if strings.TrimSpace(string(userID)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "user ID cannot be empty"}
	}
	if strings.TrimSpace(tokenHash) == "" {
		return nil, &Error{Category: InvalidInput, Message: "token hash cannot be empty"}
	}
	return &PasswordResetToken{
		id:        id,
		userID:    userID,
		tokenHash: tokenHash,
		expiresAt: expiresAt,
		createdAt: createdAt,
	}, nil
}

func RehydratePasswordResetToken(id PasswordResetID, userID UserID, tokenHash string, expiresAt time.Time, usedAt *time.Time, createdAt time.Time) *PasswordResetToken {
	return &PasswordResetToken{
		id:        id,
		userID:    userID,
		tokenHash: tokenHash,
		expiresAt: expiresAt,
		usedAt:    usedAt,
		createdAt: createdAt,
	}
}

func (p *PasswordResetToken) ID() PasswordResetID  { return p.id }
func (p *PasswordResetToken) UserID() UserID       { return p.userID }
func (p *PasswordResetToken) TokenHash() string    { return p.tokenHash }
func (p *PasswordResetToken) ExpiresAt() time.Time { return p.expiresAt }
func (p *PasswordResetToken) UsedAt() *time.Time   { return p.usedAt }
func (p *PasswordResetToken) CreatedAt() time.Time { return p.createdAt }

func (p *PasswordResetToken) IsValid(now time.Time) bool {
	return p.usedAt == nil && now.Before(p.expiresAt)
}

func (p *PasswordResetToken) MarkUsed(now time.Time) {
	p.usedAt = &now
}
