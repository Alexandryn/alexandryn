package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestReaderContentGrant_SignVerifyRoundTrip(t *testing.T) {
	s := auth.NewReaderContentGrantSigner([]byte("reader-content-grant-v1-key"), "alexandryn")
	now := time.Unix(1_700_000_000, 0)

	tok, exp, err := s.Sign("user-1", "lib-1", "ed-1", now)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !exp.Equal(now.Add(auth.ReaderContentGrantTTL)) {
		t.Errorf("expiry = %v, want now+TTL", exp)
	}
	claims, err := s.Verify(tok, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Type != auth.TokenTypeReaderContent || claims.Subject != "user-1" || claims.LibraryID != "lib-1" || claims.EditionID != "ed-1" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestReaderContentGrant_SignRequiresScope(t *testing.T) {
	s := auth.NewReaderContentGrantSigner([]byte("k"), "alexandryn")
	now := time.Unix(1_700_000_000, 0)
	cases := []struct {
		user domain.UserID
		lib  domain.LibraryID
		ed   domain.EditionID
	}{{"", "l", "e"}, {"u", "", "e"}, {"u", "l", ""}}
	for _, c := range cases {
		if _, _, err := s.Sign(c.user, c.lib, c.ed, now); err == nil {
			t.Errorf("Sign(%q,%q,%q) succeeded, want error", c.user, c.lib, c.ed)
		}
	}
}

func TestReaderContentGrant_Rejections(t *testing.T) {
	s := auth.NewReaderContentGrantSigner([]byte("key-a"), "alexandryn")
	now := time.Unix(1_700_000_000, 0)
	tok, _, _ := s.Sign("user-1", "lib-1", "ed-1", now)

	t.Run("expired", func(t *testing.T) {
		if _, err := s.Verify(tok, now.Add(auth.ReaderContentGrantTTL)); err == nil {
			t.Fatal("want expiry error")
		}
	})

	t.Run("tampered payload", func(t *testing.T) {
		parts := strings.Split(tok, ".")
		if _, err := s.Verify(parts[0]+"."+parts[1]+"x."+parts[2], now); err == nil {
			t.Fatal("want signature error")
		}
	})

	t.Run("other subkey", func(t *testing.T) {
		other := auth.NewReaderContentGrantSigner([]byte("key-b"), "alexandryn")
		if _, err := other.Verify(tok, now); err == nil {
			t.Fatal("grant verified under a different key")
		}
	})

	t.Run("issuer mismatch", func(t *testing.T) {
		other := auth.NewReaderContentGrantSigner([]byte("key-a"), "someone-else")
		if _, err := other.Verify(tok, now); err == nil {
			t.Fatal("want issuer error")
		}
	})

	t.Run("access token under same key is not a grant", func(t *testing.T) {
		access, err := auth.NewJWTSigner([]byte("key-a"), "alexandryn").Sign(auth.Claims{
			Subject:   "user-1",
			Type:      auth.TokenTypeAccess,
			ExpiresAt: now.Add(time.Hour).Unix(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Verify(access, now); err == nil {
			t.Fatal("access token accepted as a reader content grant")
		}
	})

	t.Run("grant is not an access token", func(t *testing.T) {
		if _, err := auth.NewJWTSigner([]byte("key-a"), "alexandryn").VerifyAccessToken(tok, now); err == nil {
			t.Fatal("reader content grant accepted as an access token")
		}
	})
}
