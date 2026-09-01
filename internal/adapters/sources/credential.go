package sources

import (
	"log/slog"
	"strings"
	"unicode"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// credentialPlaceholder is what a Credential shows in place of its real
// value everywhere — logs, JSON, and default formatting.
const credentialPlaceholder = "[redacted]"

const (
	maxCredentialUsernameLen = 255
	maxCredentialPasswordLen = 1024
)

// Credential is an HTTP Basic Auth username/password for an OPDS source
// (backend-source-adapter.md FR-1). It implements slog.LogValuer,
// json.Marshaler, and fmt.Stringer, each returning a fixed placeholder —
// backend-errors-and-logging.md FR-8's dual-interface mechanism, which
// explicitly anticipated "a future credential" as a covered case.
// Implementing only one interface leaves the other path open: slog's
// JSON handler falls back to encoding/json when a containing struct is
// logged as one attribute, and encoding/json knows nothing of
// slog.LogValuer.
//
// The zero Credential is "no credential". Reveal is the one method that
// returns the real values, named so every call site is a conscious
// choice.
type Credential struct {
	username string
	password string
}

// NewCredential validates and constructs a Credential. Both fields must
// be non-empty, within length bounds, and free of control characters
// (constitution §4).
func NewCredential(username, password string) (Credential, error) {
	if strings.TrimSpace(username) == "" {
		return Credential{}, &domain.Error{Category: domain.InvalidInput, Message: "credential username must not be empty"}
	}
	if strings.TrimSpace(password) == "" {
		return Credential{}, &domain.Error{Category: domain.InvalidInput, Message: "credential password must not be empty"}
	}
	if len([]rune(username)) > maxCredentialUsernameLen {
		return Credential{}, &domain.Error{Category: domain.InvalidInput, Message: "credential username is too long"}
	}
	if len([]rune(password)) > maxCredentialPasswordLen {
		return Credential{}, &domain.Error{Category: domain.InvalidInput, Message: "credential password is too long"}
	}
	if hasControlChar(username) || hasControlChar(password) {
		return Credential{}, &domain.Error{Category: domain.InvalidInput, Message: "credential contains a control character"}
	}
	return Credential{username: username, password: password}, nil
}

func hasControlChar(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// IsZero reports whether this is the "no credential" value.
func (c Credential) IsZero() bool { return c.username == "" && c.password == "" }

// Reveal returns the real username and password.
func (c Credential) Reveal() (username, password string) {
	return c.username, c.password
}

// String satisfies fmt.Stringer.
func (c Credential) String() string { return credentialPlaceholder }

// LogValue satisfies slog.LogValuer.
func (c Credential) LogValue() slog.Value { return slog.StringValue(credentialPlaceholder) }

// MarshalJSON satisfies json.Marshaler.
func (c Credential) MarshalJSON() ([]byte, error) {
	return []byte(`"` + credentialPlaceholder + `"`), nil
}
