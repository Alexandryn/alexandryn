package auth_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/auth"
)

func TestArgon2idPasswordHashing(t *testing.T) {
	hasher := auth.NewArgon2idPasswordHasher(auth.DefaultArgon2idParams())

	password := "SecretPassw0rd!123"

	t.Run("hash generation and verification", func(t *testing.T) {
		encodedHash, err := hasher.HashPassword(password)
		if err != nil {
			t.Fatalf("unexpected hash error: %v", err)
		}

		if !strings.HasPrefix(encodedHash, "$argon2id$v=19$m=65536,t=3,p=4$") {
			t.Errorf("unexpected hash prefix format: %s", encodedHash)
		}

		// Verify correct password
		match, err := hasher.VerifyPassword(password, encodedHash)
		if err != nil {
			t.Fatalf("unexpected verify error: %v", err)
		}
		if !match {
			t.Error("expected password to match hash")
		}

		// Verify incorrect password
		match, err = hasher.VerifyPassword("WrongPassword!123", encodedHash)
		if err != nil {
			t.Fatalf("unexpected verify error on mismatch: %v", err)
		}
		if match {
			t.Error("expected wrong password to not match")
		}
	})

	t.Run("malformed hash strings return error", func(t *testing.T) {
		malformed := []string{
			"",
			"plainpassword",
			"$bcrypt$v=1$invalid",
			"$argon2id$v=19$m=abc,t=3,p=4$salt$key",
			"$argon2id$v=19$m=65536,t=3,p=4$notbase64!$key",
		}

		for _, h := range malformed {
			match, err := hasher.VerifyPassword(password, h)
			if err == nil && match {
				t.Errorf("expected error or false for malformed hash %q", h)
			}
		}
	})
}
