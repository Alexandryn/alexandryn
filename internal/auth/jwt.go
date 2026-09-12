package auth

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// newJTI returns a 128-bit random token identifier as hex.
func newJTI() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("auth/jwt: generate jti: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

type Claims struct {
	Subject   domain.UserID      `json:"sub"`
	Username  string             `json:"username,omitempty"`
	Role      domain.Role        `json:"role,omitempty"`
	Libraries []domain.LibraryID `json:"libraries,omitempty"`
	JTI       string             `json:"jti,omitempty"`
	IssuedAt  int64              `json:"iat"`
	ExpiresAt int64              `json:"exp"`
	Issuer    string             `json:"iss"`
	Type      string             `json:"typ,omitempty"`
}

// Token type values for the Claims.Type field. A verifier on the
// authentication path must assert the type, not only the signature: an
// MFA ticket and a pairing enrolment grant are signed with related key
// material and carry a real Subject, so a signature-valid token minted for
// one purpose must not be accepted on another to prevent purpose confusion.
const (
	TokenTypeAccess    = "access"
	TokenTypeMFATicket = "mfa_ticket"
)

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type TokenSigner interface {
	Sign(claims Claims) (string, error)
	// Verify checks the signature, algorithm, expiry, and issuer. It does
	// NOT check the token type — callers on the authentication path use
	// VerifyAccessToken instead.
	Verify(tokenString string, now time.Time) (*Claims, error)
	// VerifyAccessToken is Verify plus a token-type assertion: it accepts
	// only an access token (empty type or TokenTypeAccess) and rejects an
	// MFA ticket, a pairing enrolment grant, or any other type even when
	// the signature is valid.
	VerifyAccessToken(tokenString string, now time.Time) (*Claims, error)
	SignMFATicket(userID domain.UserID, expiresAt time.Time) (string, error)
	// VerifyMFATicket returns the ticket's subject and its jti. The caller
	// on the MFA path claims the jti so a captured ticket cannot be
	// replayed within its TTL (#189).
	VerifyMFATicket(ticketString string, now time.Time) (userID domain.UserID, jti string, err error)
}

type JWTSigner struct {
	secret []byte
	// mfaTicketSecret signs and verifies MFA tickets only — a distinct
	// HKDF subkey of `secret`, ensuring a signature-valid access token cannot
	// verify as an MFA ticket even if type assertions are bypassed.
	mfaTicketSecret []byte
	issuer          string
}

func NewJWTSigner(secret []byte, issuer string) *JWTSigner {
	mfaKey, err := hkdf.Key(sha256.New, secret, nil, "mfa-ticket-signing-v1", 32)
	if err != nil {
		// HKDF-Expand only fails for an absurd output length; 32 bytes
		// from SHA-256 never does. Fall back to the base secret rather
		// than panic — still type-asserted, just not key-separated.
		mfaKey = secret
	}
	return &JWTSigner{
		secret:          secret,
		mfaTicketSecret: mfaKey,
		issuer:          issuer,
	}
}

func (s *JWTSigner) Sign(claims Claims) (string, error) {
	if claims.Issuer == "" {
		claims.Issuer = s.issuer
	}
	return hs256Sign(s.secret, claims)
}

func (s *JWTSigner) Verify(tokenString string, now time.Time) (*Claims, error) {
	claimsJSON, err := hs256VerifiedClaims(s.secret, tokenString)
	if err != nil {
		return nil, err
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, errors.New("auth/jwt: invalid claims JSON")
	}

	if claims.ExpiresAt <= now.Unix() {
		return nil, errors.New("auth/jwt: token expired")
	}

	if s.issuer != "" && claims.Issuer != s.issuer {
		return nil, fmt.Errorf("auth/jwt: issuer mismatch, expected %s", s.issuer)
	}

	return &claims, nil
}

// VerifyAccessToken verifies the token and asserts it is an access token.
// The type must be exactly TokenTypeAccess: tokens with any other type or
// an empty type are rejected.
func (s *JWTSigner) VerifyAccessToken(tokenString string, now time.Time) (*Claims, error) {
	claims, err := s.Verify(tokenString, now)
	if err != nil {
		return nil, err
	}
	if claims.Type != TokenTypeAccess {
		return nil, fmt.Errorf("auth/jwt: token type %q is not an access token", claims.Type)
	}
	return claims, nil
}

func (s *JWTSigner) SignMFATicket(userID domain.UserID, expiresAt time.Time) (string, error) {
	jti, err := newJTI()
	if err != nil {
		return "", err
	}
	claims := Claims{
		Subject:   userID,
		Type:      TokenTypeMFATicket,
		JTI:       jti,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: expiresAt.Unix(),
		Issuer:    s.issuer,
	}
	// Signed with the dedicated MFA-ticket subkey, ensuring the access-token
	// path cannot produce or accept this token.
	return hs256Sign(s.mfaTicketSecret, claims)
}

func (s *JWTSigner) VerifyMFATicket(ticketString string, now time.Time) (domain.UserID, string, error) {
	claimsJSON, err := hs256VerifiedClaims(s.mfaTicketSecret, ticketString)
	if err != nil {
		return "", "", err
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return "", "", errors.New("auth/jwt: invalid claims JSON")
	}
	if claims.ExpiresAt <= now.Unix() {
		return "", "", errors.New("auth/jwt: token expired")
	}
	if s.issuer != "" && claims.Issuer != s.issuer {
		return "", "", fmt.Errorf("auth/jwt: issuer mismatch, expected %s", s.issuer)
	}
	if claims.Type != TokenTypeMFATicket {
		return "", "", errors.New("auth/jwt: invalid token type for MFA ticket")
	}
	return claims.Subject, claims.JTI, nil
}
