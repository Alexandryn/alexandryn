package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// TokenTypeReaderContent is the typ of a reader-content grant: the
// credential the reader's sandboxed <iframe> carries (as a path-scoped
// HttpOnly cookie) to GET /api/v1/library/editions/{editionId}/reader/content/…
// A browser navigation cannot send an Authorization header, so the
// iframe — and every image and stylesheet the chapter pulls in by relative
// URL — needs a credential the browser attaches on its own.
//
// A grant authorises reading exactly one Edition's content, in one
// library, for one user, for ReaderContentGrantTTL. It is signed with its
// OWN HKDF subkey ("reader-content-grant-v1"), never the access-token key,
// so it can never be accepted on the access path.
const TokenTypeReaderContent = "reader_content"

// ReaderContentGrantTTL bounds a grant's life. The reader re-issues well
// before expiry for as long as it stays open.
const ReaderContentGrantTTL = 15 * time.Minute

// ReaderContentClaims is the grant's payload. Deliberately minimal: no
// role, no username, no library list.
type ReaderContentClaims struct {
	Type      string           `json:"typ"`
	Subject   domain.UserID    `json:"sub"`
	LibraryID domain.LibraryID `json:"lib"`
	EditionID domain.EditionID `json:"ed"`
	IssuedAt  int64            `json:"iat"`
	ExpiresAt int64            `json:"exp"`
	Issuer    string           `json:"iss"`
}

// ReaderContentGrantSigner mints and verifies reader-content grants.
// Construct it with the "reader-content-grant-v1" subkey.
type ReaderContentGrantSigner struct {
	secret []byte
	issuer string
}

func NewReaderContentGrantSigner(secret []byte, issuer string) *ReaderContentGrantSigner {
	return &ReaderContentGrantSigner{secret: secret, issuer: issuer}
}

// Sign mints a grant for userID to read editionID in libraryID, and
// returns it with its expiry.
func (s *ReaderContentGrantSigner) Sign(userID domain.UserID, libraryID domain.LibraryID, editionID domain.EditionID, now time.Time) (string, time.Time, error) {
	if userID == "" || libraryID == "" || editionID == "" {
		return "", time.Time{}, errors.New("auth: reader content grant needs a user, library, and edition")
	}
	expiresAt := now.Add(ReaderContentGrantTTL)
	token, err := hs256Sign(s.secret, ReaderContentClaims{
		Type:      TokenTypeReaderContent,
		Subject:   userID,
		LibraryID: libraryID,
		EditionID: editionID,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		Issuer:    s.issuer,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Verify checks the algorithm, signature (constant time), expiry, issuer,
// that typ is exactly "reader_content", and that every scope claim is
// present. Matching the grant's edition against the requested one is the
// caller's job.
func (s *ReaderContentGrantSigner) Verify(token string, now time.Time) (*ReaderContentClaims, error) {
	claimsJSON, err := hs256VerifiedClaims(s.secret, token)
	if err != nil {
		return nil, err
	}
	var claims ReaderContentClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, errors.New("auth: malformed reader content grant claims")
	}
	if claims.Type != TokenTypeReaderContent {
		return nil, fmt.Errorf("auth: token type %q is not a reader content grant", claims.Type)
	}
	if claims.ExpiresAt <= now.Unix() {
		return nil, errors.New("auth: reader content grant expired")
	}
	if s.issuer != "" && claims.Issuer != s.issuer {
		return nil, errors.New("auth: reader content grant issuer mismatch")
	}
	if claims.Subject == "" || claims.LibraryID == "" || claims.EditionID == "" {
		return nil, errors.New("auth: reader content grant is missing a required claim")
	}
	return &claims, nil
}
