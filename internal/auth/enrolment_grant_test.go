package auth_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
)

type seqIDs struct{ n int }

func (s *seqIDs) NewID() string { s.n++; return "jti-" + strconv.Itoa(s.n) }

func newGrantSigner(secret string) *auth.EnrolmentGrantSigner {
	return auth.NewEnrolmentGrantSigner([]byte(secret), "alexandryn", &seqIDs{})
}

func TestEnrolmentGrant_SignVerifyRoundTrip(t *testing.T) {
	s := newGrantSigner("enrolment-grant-v1-key-material")
	now := time.Unix(1_700_000_000, 0)

	tok, err := s.Sign("ps-42", now)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	claims, err := s.Verify(tok, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Type != auth.TokenTypeEnrolment {
		t.Errorf("typ = %q, want %q", claims.Type, auth.TokenTypeEnrolment)
	}
	if claims.SessionID != "ps-42" {
		t.Errorf("sid = %q", claims.SessionID)
	}
	if claims.JTI == "" {
		t.Error("jti is empty")
	}
	// No UserID / role anywhere in the token.
	if strings.Contains(tok, "sub") || strings.Contains(tok, "role") {
		t.Errorf("grant token carries user identity: %s", tok)
	}
}

func TestEnrolmentGrant_Rejections(t *testing.T) {
	s := newGrantSigner("key-a")
	now := time.Unix(1_700_000_000, 0)
	tok, _ := s.Sign("ps-1", now)

	t.Run("expired", func(t *testing.T) {
		if _, err := s.Verify(tok, now.Add(auth.EnrolmentGrantTTL+time.Second)); err == nil {
			t.Fatal("want expiry error")
		}
	})

	t.Run("tampered payload", func(t *testing.T) {
		parts := strings.Split(tok, ".")
		bad := parts[0] + "." + parts[1] + "x." + parts[2]
		if _, err := s.Verify(bad, now); err == nil {
			t.Fatal("want signature error")
		}
	})

	t.Run("wrong subkey", func(t *testing.T) {
		other := newGrantSigner("key-b")
		if _, err := other.Verify(tok, now); err == nil {
			t.Fatal("a grant signed with a different subkey must not verify")
		}
	})

	t.Run("an access-shaped token is not an enrolment grant", func(t *testing.T) {
		jwt := auth.NewJWTSigner([]byte("key-a"), "alexandryn")
		access, _ := jwt.Sign(auth.Claims{Subject: "u1", Type: auth.TokenTypeAccess, ExpiresAt: now.Add(time.Hour).Unix(), Issuer: "alexandryn"})
		if _, err := s.Verify(access, now); err == nil {
			t.Fatal("an access token must be rejected by the enrolment grant verifier (wrong typ)")
		}
	})

	t.Run("an enrolment grant is rejected on the access path", func(t *testing.T) {
		// The grant verifier and VerifyAccessToken use different keys, but
		// even with a shared key the typ check rejects it.
		jwt := auth.NewJWTSigner([]byte("key-a"), "alexandryn")
		if _, err := jwt.VerifyAccessToken(tok, now); err == nil {
			t.Fatal("an enrolment grant must be rejected on the access path")
		}
	})
}
