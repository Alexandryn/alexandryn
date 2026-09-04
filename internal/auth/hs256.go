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
)

// The HS256 / compact-JWT wire format, in one place. Both the access-token
// signer (JWTSigner) and the pairing enrolment-grant signer use it — the
// claims structs and the type/expiry assertions differ, the envelope does
// not, and two copies drift (a future header-typ or length cap would have
// to be made in both). Neither the header nor the payload is a place for
// untrusted structure: this file only encodes/decodes and verifies the
// MAC; expiry, issuer and token-type checks stay with each caller.

func hmacSHA256(secret, data []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write(data)
	return m.Sum(nil)
}

// hs256Sign marshals a fixed {alg:HS256, typ:JWT} header and claims,
// base64url-encodes both, and appends the MAC.
func hs256Sign(secret []byte, claims any) (string, error) {
	headerJSON, err := json.Marshal(jwtHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", fmt.Errorf("auth: marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("auth: marshal claims: %w", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(hmacSHA256(secret, []byte(signingInput))), nil
}

// hs256VerifiedClaims splits a compact token, rejects any alg but HS256,
// verifies the MAC in constant time, and returns the decoded claims JSON.
// It performs NO semantic checks — the caller unmarshals into its own
// claims type and asserts expiry / issuer / token type.
func hs256VerifiedClaims(secret []byte, token string) ([]byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("auth: invalid token format")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("auth: invalid header encoding")
	}
	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, errors.New("auth: invalid header JSON")
	}
	if header.Algorithm != "HS256" {
		return nil, fmt.Errorf("auth: unsupported algorithm %q", header.Algorithm)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("auth: invalid signature encoding")
	}
	if subtle.ConstantTimeCompare(sig, hmacSHA256(secret, []byte(parts[0]+"."+parts[1]))) != 1 {
		return nil, errors.New("auth: signature mismatch")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("auth: invalid claims encoding")
	}
	return claimsJSON, nil
}
