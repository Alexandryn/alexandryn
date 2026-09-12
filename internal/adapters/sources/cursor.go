package sources

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Cursor is the decoded form of a browse/search continuation token.
// Position is a resumable sort key, never concatenated into a filesystem path
// or fetched without an origin check — a last-seen filename for local-folder,
// a feed's own next-page URL for opds (which requires origin re-validation against
// the source's configured origin before use).
type Cursor struct {
	SourceID string `json:"s"`
	Kind     Kind   `json:"k"`
	Position string `json:"p"`
}

// CursorCodec signs and verifies cursors with an HMAC key (in practice
// an HKDF subkey of the credential key — see crypto.Service.DeriveSubkey)
// so a client cannot forge or tamper with a continuation token. The
// token is base64url( 32-byte HMAC-SHA256 || JSON payload ).
type CursorCodec struct {
	key []byte
}

// NewCursorCodec builds a codec from an HMAC key.
func NewCursorCodec(hmacKey []byte) *CursorCodec {
	dup := make([]byte, len(hmacKey))
	copy(dup, hmacKey)
	return &CursorCodec{key: dup}
}

const cursorMACLen = sha256.Size

// Encode returns the opaque token for c.
func (cc *CursorCodec) Encode(c Cursor) string {
	payload, _ := json.Marshal(c) // Cursor has only string fields — cannot fail
	mac := hmac.New(sha256.New, cc.key)
	mac.Write(payload)
	sum := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(append(sum, payload...))
}

// Decode verifies token's signature and returns the Cursor. A tampered,
// truncated, or foreign token — and a token whose SourceID does not
// match wantSourceID — is rejected with an InvalidInput *domain.Error.
// Re-checking wantSourceID means a cursor minted for one source can
// never be replayed against another.
func (cc *CursorCodec) Decode(token, wantSourceID string) (Cursor, error) {
	invalid := &domain.Error{Category: domain.InvalidInput, Message: "cursor is invalid"}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < cursorMACLen+1 {
		return Cursor{}, invalid
	}
	gotMAC, payload := raw[:cursorMACLen], raw[cursorMACLen:]

	mac := hmac.New(sha256.New, cc.key)
	mac.Write(payload)
	wantMAC := mac.Sum(nil)
	if subtle.ConstantTimeCompare(gotMAC, wantMAC) != 1 {
		return Cursor{}, invalid
	}

	var c Cursor
	if err := json.Unmarshal(payload, &c); err != nil {
		return Cursor{}, invalid
	}
	if c.SourceID != wantSourceID {
		return Cursor{}, invalid
	}
	if c.Kind != KindLocalFolder && c.Kind != KindOPDS {
		return Cursor{}, invalid
	}
	return c, nil
}

// ForOPDSPosition validates that a decoded opds cursor's position is a
// same-origin URL relative to baseURL before it is fetched, and
// returns it. An off-origin position is rejected with InvalidInput —
// never fetched.
func (c Cursor) ForOPDSPosition(baseURL string) (string, error) {
	if c.Kind != KindOPDS {
		return "", fmt.Errorf("cursor kind %q is not opds", c.Kind)
	}
	if c.Position == "" {
		return "", nil
	}
	if !SameOrigin(baseURL, c.Position) {
		return "", &domain.Error{Category: domain.InvalidInput, Message: "cursor points off-origin"}
	}
	return c.Position, nil
}
