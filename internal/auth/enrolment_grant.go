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

// TokenTypeEnrolment is the typ of a device-pairing enrolment grant
// (backend-network-transport.md FR-10, ADR 0028 §6). It is minted by
// POST /api/v1/network/pair/verify and spent once at
// POST /api/v1/auth/login to associate the new session's device with the
// PairingSession. It carries NO UserID and NO role — it authorises
// exactly that one association step and nothing else.
//
// A grant is signed with its OWN HKDF subkey ("enrolment-grant-v1"),
// never the access-token key, so it is structurally impossible for it to
// be accepted on the access path — VerifyAccessToken's type check AND the
// distinct key both reject it (the phase-12 MFA-ticket-as-bearer defect,
// AUDIT-0012-C2, is not repeated).
const TokenTypeEnrolment = "enrol"

// EnrolmentGrantTTL is fixed at 10 minutes — the flow is "type your
// password on a TV keyboard", which is slow. It is never widened per
// call.
const EnrolmentGrantTTL = 10 * time.Minute

// EnrolmentClaims is the grant's payload. Deliberately minimal.
type EnrolmentClaims struct {
	Type      string                  `json:"typ"`
	SessionID domain.PairingSessionID `json:"sid"`
	JTI       string                  `json:"jti"`
	IssuedAt  int64                   `json:"iat"`
	ExpiresAt int64                   `json:"exp"`
	Issuer    string                  `json:"iss"`
}

// EnrolmentGrantSigner mints and verifies enrolment grants. Construct it
// with the "enrolment-grant-v1" subkey.
type EnrolmentGrantSigner struct {
	secret []byte
	issuer string
	ids    domain.IDGenerator
}

func NewEnrolmentGrantSigner(secret []byte, issuer string, ids domain.IDGenerator) *EnrolmentGrantSigner {
	return &EnrolmentGrantSigner{secret: secret, issuer: issuer, ids: ids}
}

// Sign mints a grant for sessionID. The jti is fresh per call; the caller
// records it as spent at login time (single-use, backend-network-api.md
// FR-9) — this function persists nothing.
func (s *EnrolmentGrantSigner) Sign(sessionID domain.PairingSessionID, now time.Time) (string, error) {
	if sessionID == "" {
		return "", errors.New("auth: enrolment grant needs a pairing session ID")
	}
	claims := EnrolmentClaims{
		Type:      TokenTypeEnrolment,
		SessionID: sessionID,
		JTI:       s.ids.NewID(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(EnrolmentGrantTTL).Unix(),
		Issuer:    s.issuer,
	}
	return s.encode(claims)
}

// Verify checks the algorithm, signature (constant time), expiry, issuer,
// and that typ is exactly "enrol". An expired / tampered / wrong-subkey /
// wrong-typ grant is rejected — the caller (the login handler) treats a
// rejection as "no device association", not "login failed"
// (backend-network-api.md FR-9).
func (s *EnrolmentGrantSigner) Verify(token string, now time.Time) (*EnrolmentClaims, error) {
	h, p, sig, ok := splitJWT(token)
	if !ok {
		return nil, errors.New("auth: malformed enrolment grant")
	}

	var header jwtHeader
	if hj, err := base64.RawURLEncoding.DecodeString(h); err != nil || json.Unmarshal(hj, &header) != nil {
		return nil, errors.New("auth: malformed enrolment grant header")
	}
	if header.Algorithm != "HS256" {
		return nil, fmt.Errorf("auth: enrolment grant algorithm %q unsupported", header.Algorithm)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return nil, errors.New("auth: malformed enrolment grant signature")
	}
	if subtle.ConstantTimeCompare(sigBytes, s.hmac(h+"."+p)) != 1 {
		return nil, errors.New("auth: enrolment grant signature mismatch")
	}

	var claims EnrolmentClaims
	if cj, err := base64.RawURLEncoding.DecodeString(p); err != nil || json.Unmarshal(cj, &claims) != nil {
		return nil, errors.New("auth: malformed enrolment grant claims")
	}
	if claims.Type != TokenTypeEnrolment {
		return nil, fmt.Errorf("auth: token type %q is not an enrolment grant", claims.Type)
	}
	if claims.ExpiresAt <= now.Unix() {
		return nil, errors.New("auth: enrolment grant expired")
	}
	if s.issuer != "" && claims.Issuer != s.issuer {
		return nil, errors.New("auth: enrolment grant issuer mismatch")
	}
	if claims.SessionID == "" || claims.JTI == "" {
		return nil, errors.New("auth: enrolment grant is missing a required claim")
	}
	return &claims, nil
}

func (s *EnrolmentGrantSigner) encode(claims EnrolmentClaims) (string, error) {
	hj, err := json.Marshal(jwtHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", err
	}
	cj, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(hj) + "." + base64.RawURLEncoding.EncodeToString(cj)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(s.hmac(signingInput)), nil
}

func (s *EnrolmentGrantSigner) hmac(data string) []byte {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func splitJWT(token string) (h, p, sig string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
