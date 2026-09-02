package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

type Claims struct {
	Subject   domain.UserID       `json:"sub"`
	Username  string              `json:"username,omitempty"`
	Role      domain.Role         `json:"role,omitempty"`
	Libraries []domain.LibraryID  `json:"libraries,omitempty"`
	JTI       string              `json:"jti,omitempty"`
	IssuedAt  int64               `json:"iat"`
	ExpiresAt int64               `json:"exp"`
	Issuer    string              `json:"iss"`
	Type      string              `json:"typ,omitempty"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type TokenSigner interface {
	Sign(claims Claims) (string, error)
	Verify(tokenString string, now time.Time) (*Claims, error)
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

	header := jwtHeader{
		Algorithm: "HS256",
		Type:      "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("auth/jwt: marshal header failed: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("auth/jwt: marshal claims failed: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	sig := s.computeHMAC([]byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, nil
}

func (s *JWTSigner) Verify(tokenString string, now time.Time) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("auth/jwt: invalid token format")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("auth/jwt: invalid header encoding")
	}

	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, errors.New("auth/jwt: invalid header JSON")
	}

	// Strictly enforce HS256 algorithm (block 'none' algorithm attacks)
	if header.Algorithm != "HS256" {
		return nil, fmt.Errorf("auth/jwt: unsupported algorithm %q", header.Algorithm)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("auth/jwt: invalid signature encoding")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSig := s.computeHMAC([]byte(signingInput))

	if subtle.ConstantTimeCompare(signature, expectedSig) != 1 {
		return nil, errors.New("auth/jwt: signature mismatch")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("auth/jwt: invalid claims encoding")
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

func (s *JWTSigner) SignMFATicket(userID domain.UserID, expiresAt time.Time) (string, error) {
	claims := Claims{
		Subject:   userID,
		Type:      "mfa_ticket",
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
	if claims.Type != "mfa_ticket" {
		return "", errors.New("auth/jwt: invalid token type for MFA ticket")
	}
	return claims.Subject, nil
}

func (s *JWTSigner) computeHMAC(data []byte) []byte {
	h := hmac.New(sha256.New, s.secret)
	h.Write(data)
	return h.Sum(nil)
}
