package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// TokenTypeEnrolment is the typ of a device-pairing enrolment grant.
// It is minted by POST /api/v1/network/pair/verify and spent once at
// POST /api/v1/auth/login to associate the new session's device with the
// PairingSession. It carries NO UserID and NO role — it authorises
// exactly that one association step and nothing else.
//
// A grant is signed with its OWN HKDF subkey ("enrolment-grant-v1"),
// never the access-token key, so it is structurally impossible for it to
// be accepted on the access path — VerifyAccessToken's type check AND the
// distinct key both reject it.
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
// records it as spent at login time for single-use replay protection.
func (s *EnrolmentGrantSigner) Sign(sessionID domain.PairingSessionID, now time.Time) (string, error) {
	if sessionID == "" {
		return "", errors.New("auth: enrolment grant needs a pairing session ID")
	}
	return hs256Sign(s.secret, EnrolmentClaims{
		Type:      TokenTypeEnrolment,
		SessionID: sessionID,
		JTI:       s.ids.NewID(),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(EnrolmentGrantTTL).Unix(),
		Issuer:    s.issuer,
	})
}

// Verify checks the algorithm, signature (constant time), expiry, issuer,
// and that typ is exactly "enrol". An expired, tampered, wrong-subkey, or
// wrong-typ grant is rejected — the caller (the login handler) treats a
// rejection as "no device association", not "login failed".
func (s *EnrolmentGrantSigner) Verify(token string, now time.Time) (*EnrolmentClaims, error) {
	claimsJSON, err := hs256VerifiedClaims(s.secret, token)
	if err != nil {
		return nil, err
	}
	var claims EnrolmentClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
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
