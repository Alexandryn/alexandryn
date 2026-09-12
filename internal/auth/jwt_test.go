package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestJWTSignerAndVerifier(t *testing.T) {
	secret := []byte("super-secret-key-that-is-at-least-32-bytes-long!")
	signer := auth.NewJWTSigner(secret, "alexandryn")

	now := time.Now().Truncate(time.Second)
	claims := auth.Claims{
		Subject:   "u-12345",
		Username:  "alex",
		Role:      domain.RoleAdmin,
		Libraries: []domain.LibraryID{domain.DefaultLibraryID, "lib-2"},
		JTI:       "token-jti-1",
		Type:      auth.TokenTypeAccess,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(15 * time.Minute).Unix(),
		Issuer:    "alexandryn",
	}

	t.Run("sign and verify valid token", func(t *testing.T) {
		tokenStr, err := signer.Sign(claims)
		if err != nil {
			t.Fatalf("unexpected sign error: %v", err)
		}

		parts := strings.Split(tokenStr, ".")
		if len(parts) != 3 {
			t.Fatalf("expected 3 parts in JWT, got %d", len(parts))
		}

		verified, err := signer.Verify(tokenStr, now)
		if err != nil {
			t.Fatalf("unexpected verify error: %v", err)
		}

		if verified.Subject != claims.Subject {
			t.Errorf("expected subject %q, got %q", claims.Subject, verified.Subject)
		}
		if verified.Username != claims.Username {
			t.Errorf("expected username %q, got %q", claims.Username, verified.Username)
		}
		if verified.Role != domain.RoleAdmin {
			t.Errorf("expected role admin, got %v", verified.Role)
		}
		if len(verified.Libraries) != 2 || verified.Libraries[0] != domain.DefaultLibraryID {
			t.Errorf("expected libraries match, got %v", verified.Libraries)
		}
	})

	t.Run("expired token fails verification", func(t *testing.T) {
		expiredClaims := claims
		expiredClaims.ExpiresAt = now.Add(-1 * time.Minute).Unix()

		tokenStr, err := signer.Sign(expiredClaims)
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}

		_, err = signer.Verify(tokenStr, now)
		if err == nil {
			t.Error("expected error for expired token, got nil")
		}
	})

	t.Run("tampered payload fails verification", func(t *testing.T) {
		tokenStr, err := signer.Sign(claims)
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}

		parts := strings.Split(tokenStr, ".")
		tampered := parts[0] + ".eyJzdWIiOiJoYWNrZWQifQ." + parts[2]

		_, err = signer.Verify(tampered, now)
		if err == nil {
			t.Error("expected error for tampered token, got nil")
		}
	})

	t.Run("wrong secret fails verification", func(t *testing.T) {
		tokenStr, err := signer.Sign(claims)
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}

		wrongSigner := auth.NewJWTSigner([]byte("different-secret-key-32-bytes-long!"), "alexandryn")
		_, err = wrongSigner.Verify(tokenStr, now)
		if err == nil {
			t.Error("expected error with wrong secret, got nil")
		}
	})

	t.Run("alg none attack rejected", func(t *testing.T) {
		// Header with alg: none
		noneHeader := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0"
		payload := "eyJzdWIiOiJ1LTEyMzQ1IiwiaXNzIjoiYWxleGFuZHJ5biIsImV4cCI6OTk5OTk5OTk5OX0"
		noneToken := noneHeader + "." + payload + "."

		_, err := signer.Verify(noneToken, now)
		if err == nil {
			t.Error("expected alg: none token to be rejected")
		}
	})

	t.Run("VerifyAccessToken accepts an explicit access token", func(t *testing.T) {
		tokenStr, err := signer.Sign(claims)
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}
		if _, err := signer.VerifyAccessToken(tokenStr, now); err != nil {
			t.Errorf("expected typ:access token to verify, got %v", err)
		}
	})

	t.Run("VerifyAccessToken rejects an empty token type (#173)", func(t *testing.T) {
		untyped := claims
		untyped.Type = ""
		tokenStr, err := signer.Sign(untyped)
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}
		if _, err := signer.VerifyAccessToken(tokenStr, now); err == nil {
			t.Error("expected an empty-type token to be rejected on the access path")
		}
	})

	t.Run("VerifyAccessToken rejects a signature-valid MFA ticket", func(t *testing.T) {
		// An MFA ticket carries a real Subject and typ:mfa_ticket. The access
		// path must not accept it: the type is asserted and the ticket is signed
		// with a distinct HKDF subkey.
		ticket, err := signer.SignMFATicket("u-12345", now.Add(5*time.Minute))
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}
		// Signed with the MFA-ticket subkey, so a raw access-key verify fails.
		if _, err := signer.Verify(ticket, now); err == nil {
			t.Error("expected an MFA ticket NOT to verify against the access secret")
		}
		// And the access path rejects it.
		if _, err := signer.VerifyAccessToken(ticket, now); err == nil {
			t.Error("expected VerifyAccessToken to reject an MFA ticket, got nil")
		}
		// But it does verify as an MFA ticket.
		if _, _, err := signer.VerifyMFATicket(ticket, now); err != nil {
			t.Errorf("MFA ticket should verify via VerifyMFATicket: %v", err)
		}
	})

	t.Run("VerifyAccessToken rejects an unknown token type", func(t *testing.T) {
		enrol := claims
		enrol.Type = "enrol"
		tokenStr, err := signer.Sign(enrol)
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}
		if _, err := signer.VerifyAccessToken(tokenStr, now); err == nil {
			t.Error("expected VerifyAccessToken to reject typ:enrol, got nil")
		}
	})

	t.Run("VerifyAccessToken rejects an expired token", func(t *testing.T) {
		expired := claims
		expired.ExpiresAt = now.Add(-time.Minute).Unix()
		tokenStr, err := signer.Sign(expired)
		if err != nil {
			t.Fatalf("sign error: %v", err)
		}
		if _, err := signer.VerifyAccessToken(tokenStr, now); err == nil {
			t.Error("expected VerifyAccessToken to reject an expired token, got nil")
		}
	})

	t.Run("MFATicket signing and verification", func(t *testing.T) {
		ticket, err := signer.SignMFATicket("u-12345", now.Add(5*time.Minute))
		if err != nil {
			t.Fatalf("unexpected MFA ticket sign error: %v", err)
		}

		userID, jti, err := signer.VerifyMFATicket(ticket, now)
		if err != nil {
			t.Fatalf("unexpected MFA ticket verify error: %v", err)
		}
		if userID != "u-12345" {
			t.Errorf("expected user ID u-12345, got %v", userID)
		}
		if jti == "" {
			t.Error("expected a non-empty jti on the MFA ticket (#189)")
		}

		// A second ticket carries a distinct jti.
		ticket2, _ := signer.SignMFATicket("u-12345", now.Add(5*time.Minute))
		_, jti2, _ := signer.VerifyMFATicket(ticket2, now)
		if jti2 == jti {
			t.Error("expected distinct jtis on distinct MFA tickets")
		}

		// Expired ticket
		_, _, err = signer.VerifyMFATicket(ticket, now.Add(10*time.Minute))
		if err == nil {
			t.Error("expected error for expired MFA ticket")
		}
	})
}
