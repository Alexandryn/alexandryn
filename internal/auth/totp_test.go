package auth_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
)

func TestTOTPGenerationAndVerification(t *testing.T) {
	totpEngine := auth.NewTOTPEngine("Alexandryn")

	t.Run("generate secret and verify codes", func(t *testing.T) {
		secret, err := totpEngine.GenerateSecret()
		if err != nil {
			t.Fatalf("unexpected generate secret error: %v", err)
		}

		if len(secret) < 16 {
			t.Errorf("secret too short: %s", secret)
		}

		now := time.Now()
		code, err := totpEngine.GenerateCode(secret, now)
		if err != nil {
			t.Fatalf("unexpected generate code error: %v", err)
		}

		if len(code) != 6 {
			t.Errorf("expected 6-digit code, got %s", code)
		}

		// Verify current code
		valid, err := totpEngine.ValidateCode(secret, code, now)
		if err != nil {
			t.Fatalf("unexpected validate error: %v", err)
		}
		if !valid {
			t.Error("expected valid code to pass")
		}

		// Verify code in -1 step window (25s ago)
		valid, err = totpEngine.ValidateCode(secret, code, now.Add(25*time.Second))
		if err != nil || !valid {
			t.Errorf("expected code to be valid in +/- 1 step window, got valid=%v, err=%v", valid, err)
		}

		// Verify code far in past fails
		valid, err = totpEngine.ValidateCode(secret, code, now.Add(90*time.Second))
		if err != nil || valid {
			t.Errorf("expected old code to be rejected, got valid=%v, err=%v", valid, err)
		}

		// Invalid code fails
		valid, err = totpEngine.ValidateCode(secret, "000000", now)
		if valid {
			// unless 000000 happened to be the code
			if code != "000000" {
				t.Error("expected random 000000 code to fail")
			}
		}
	})

	t.Run("generate Key URI", func(t *testing.T) {
		uri := totpEngine.KeyURI("alex@example.com", "JBSWY3DPEHPK3PXP")
		expected := "otpauth://totp/Alexandryn:alex@example.com?algorithm=SHA1&digits=6&issuer=Alexandryn&period=30&secret=JBSWY3DPEHPK3PXP"
		if uri != expected {
			t.Errorf("expected %q, got %q", expected, uri)
		}
	})

	t.Run("recovery codes generation and hashing", func(t *testing.T) {
		codes, hashes, err := totpEngine.GenerateRecoveryCodes(8)
		if err != nil {
			t.Fatalf("unexpected generate recovery codes error: %v", err)
		}

		if len(codes) != 8 || len(hashes) != 8 {
			t.Fatalf("expected 8 codes and hashes, got %d, %d", len(codes), len(hashes))
		}

		// Verify valid recovery code
		matchIdx := totpEngine.VerifyRecoveryCode(codes[0], hashes)
		if matchIdx != 0 {
			t.Errorf("expected match at index 0, got %d", matchIdx)
		}

		// Verify invalid recovery code
		invalidIdx := totpEngine.VerifyRecoveryCode("INVALID-CODE", hashes)
		if invalidIdx != -1 {
			t.Errorf("expected -1 for invalid code, got %d", invalidIdx)
		}
	})

	t.Run("AES-256-GCM secret encryption and decryption", func(t *testing.T) {
		key := []byte("master-encryption-key-32-bytes!!")
		plainSecret := "JBSWY3DPEHPK3PXP"

		ciphertext, err := auth.EncryptSecret([]byte(plainSecret), key)
		if err != nil {
			t.Fatalf("unexpected encrypt error: %v", err)
		}

		decrypted, err := auth.DecryptSecret(ciphertext, key)
		if err != nil {
			t.Fatalf("unexpected decrypt error: %v", err)
		}

		if string(decrypted) != plainSecret {
			t.Errorf("expected %q, got %q", plainSecret, string(decrypted))
		}

		// Decrypt with wrong key fails
		_, err = auth.DecryptSecret(ciphertext, []byte("wrong-encryption-key-32-bytes!!"))
		if err == nil {
			t.Error("expected decryption to fail with wrong key")
		}
	})
}
