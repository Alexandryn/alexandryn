package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestRoleValidation(t *testing.T) {
	tests := []struct {
		input   string
		valid   bool
		expRole domain.Role
	}{
		{"admin", true, domain.RoleAdmin},
		{"reader", true, domain.RoleReader},
		{"ADMIN", false, ""},
		{"superuser", false, ""},
		{"", false, ""},
	}

	for _, tt := range tests {
		role, err := domain.ParseRole(tt.input)
		if tt.valid {
			if err != nil {
				t.Errorf("expected valid role for %q, got error: %v", tt.input, err)
			}
			if role != tt.expRole {
				t.Errorf("expected %v, got %v", tt.expRole, role)
			}
			if !role.IsValid() {
				t.Errorf("expected role %v to be valid", role)
			}
		} else {
			if err == nil {
				t.Errorf("expected error for invalid role %q, got nil", tt.input)
			}
		}
	}
}

func TestNewUser(t *testing.T) {
	now := time.Now()
	validID := domain.UserID("u-123")

	t.Run("valid user construction", func(t *testing.T) {
		user, err := domain.NewUser(validID, "alex", "alex@example.com", domain.RoleAdmin, now, now)
		if err != nil {
			t.Fatalf("expected valid user, got: %v", err)
		}
		if user.ID() != validID {
			t.Errorf("expected ID %v, got %v", validID, user.ID())
		}
		if user.Username() != "alex" {
			t.Errorf("expected username alex, got %v", user.Username())
		}
		if user.Email() != "alex@example.com" {
			t.Errorf("expected email alex@example.com, got %v", user.Email())
		}
		if user.Role() != domain.RoleAdmin {
			t.Errorf("expected role admin, got %v", user.Role())
		}
	})

	t.Run("empty ID fails", func(t *testing.T) {
		_, err := domain.NewUser("", "alex", "alex@example.com", domain.RoleAdmin, now, now)
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
	})

	t.Run("invalid username fails", func(t *testing.T) {
		invalidUsernames := []string{"", "a", "ab", "user with space", "user@special!"}
		for _, uname := range invalidUsernames {
			_, err := domain.NewUser(validID, uname, "alex@example.com", domain.RoleAdmin, now, now)
			if err == nil {
				t.Errorf("expected error for username %q, got nil", uname)
			}
		}
	})

	t.Run("invalid email fails", func(t *testing.T) {
		invalidEmails := []string{"", "noatsign", "plainaddress", "@domain.com", "user@"}
		for _, email := range invalidEmails {
			_, err := domain.NewUser(validID, "alex", email, domain.RoleAdmin, now, now)
			if err == nil {
				t.Errorf("expected error for email %q, got nil", email)
			}
		}
	})

	t.Run("invalid role fails", func(t *testing.T) {
		_, err := domain.NewUser(validID, "alex", "alex@example.com", domain.Role("invalid"), now, now)
		if err == nil {
			t.Error("expected error for invalid role, got nil")
		}
	})
}

func TestRefreshTokenLifecycle(t *testing.T) {
	now := time.Now()
	tokenID := domain.RefreshTokenID("rt-123")
	userID := domain.UserID("u-123")
	hash := "sha256hashstring"
	expiresAt := now.Add(24 * time.Hour)

	rt, err := domain.NewRefreshToken(tokenID, userID, hash, expiresAt, now)
	if err != nil {
		t.Fatalf("expected valid refresh token, got: %v", err)
	}

	if !rt.IsActive(now) {
		t.Error("expected token to be active now")
	}

	// Expired check
	if rt.IsActive(now.Add(25 * time.Hour)) {
		t.Error("expected token to be inactive after expiry")
	}

	// Revoke check
	rt.Revoke(now.Add(1 * time.Hour))
	if rt.IsActive(now) {
		t.Error("expected revoked token to be inactive")
	}
	if rt.RevokedAt() == nil {
		t.Error("expected RevokedAt to be non-nil")
	}
}

func TestTOTPSettingsLifecycle(t *testing.T) {
	now := time.Now()
	userID := domain.UserID("u-123")
	secret := []byte("encrypted-secret-bytes")
	recoveryHashes := []string{"hash1", "hash2"}

	totp, err := domain.NewTOTPSettings(userID, secret, recoveryHashes, false, nil, now)
	if err != nil {
		t.Fatalf("expected valid TOTPSettings, got: %v", err)
	}

	if totp.IsEnabled() {
		t.Error("expected initially not enabled")
	}

	totp.Confirm(now)
	if !totp.IsEnabled() {
		t.Error("expected enabled after confirm")
	}
	if totp.ConfirmedAt() == nil {
		t.Error("expected confirmedAt to be non-nil")
	}

	totp.Disable()
	if totp.IsEnabled() {
		t.Error("expected disabled after Disable()")
	}
}

func TestPasswordResetToken(t *testing.T) {
	now := time.Now()
	id := domain.PasswordResetID("pr-123")
	userID := domain.UserID("u-123")
	hash := "tokenhash"
	expiresAt := now.Add(1 * time.Hour)

	prt, err := domain.NewPasswordResetToken(id, userID, hash, expiresAt, now)
	if err != nil {
		t.Fatalf("expected valid PasswordResetToken, got: %v", err)
	}

	if !prt.IsValid(now) {
		t.Error("expected token to be valid now")
	}

	if prt.IsValid(now.Add(2 * time.Hour)) {
		t.Error("expected expired token to be invalid")
	}

	prt.MarkUsed(now.Add(10 * time.Minute))
	if prt.IsValid(now) {
		t.Error("expected used token to be invalid")
	}
}
