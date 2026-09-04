package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

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
// MFA ticket and (from phase 13) a pairing enrolment grant are signed
// with related key material and carry a real Subject, so a signature-valid
// token minted for one purpose must not be accepted on another
// (AUDIT-0012-C2, CLAUDE.md token-type Reflex). An access token carries
// either an empty type (the phase-12 shape) or TokenTypeAccess.
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
	VerifyMFATicket(ticketString string, now time.Time) (domain.UserID, error)
}

type JWTSigner struct {
	secret []byte
	issuer string
}

func NewJWTSigner(secret []byte, issuer string) *JWTSigner {
	return &JWTSigner{
		secret: secret,
		issuer: issuer,
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
// A signature-valid token whose type is "mfa_ticket", "enrol", or anything
// other than "" / TokenTypeAccess is rejected (AUDIT-0012-C2).
func (s *JWTSigner) VerifyAccessToken(tokenString string, now time.Time) (*Claims, error) {
	claims, err := s.Verify(tokenString, now)
	if err != nil {
		return nil, err
	}
	if claims.Type != "" && claims.Type != TokenTypeAccess {
		return nil, fmt.Errorf("auth/jwt: token type %q is not an access token", claims.Type)
	}
	return claims, nil
}

func (s *JWTSigner) SignMFATicket(userID domain.UserID, expiresAt time.Time) (string, error) {
	claims := Claims{
		Subject:   userID,
		Type:      TokenTypeMFATicket,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: expiresAt.Unix(),
		Issuer:    s.issuer,
	}
	return s.Sign(claims)
}

func (s *JWTSigner) VerifyMFATicket(ticketString string, now time.Time) (domain.UserID, error) {
	claims, err := s.Verify(ticketString, now)
	if err != nil {
		return "", err
	}
	if claims.Type != TokenTypeMFATicket {
		return "", errors.New("auth/jwt: invalid token type for MFA ticket")
	}
	return claims.Subject, nil
}
