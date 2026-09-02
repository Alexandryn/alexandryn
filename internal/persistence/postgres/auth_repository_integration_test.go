//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestUserRepository_CRUD(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	user, err := domain.NewUser("u-1", "alex", "alex@example.com", domain.RoleAdmin, now, now)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}

	// Save
	if err := userRepo.Save(ctx, user); err != nil {
		t.Fatalf("Save user: %v", err)
	}

	// FindByID
	got, err := userRepo.FindByID(ctx, "u-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Username() != "alex" || got.Email() != "alex@example.com" || got.Role() != domain.RoleAdmin {
		t.Fatalf("got = %+v", got)
	}

	// FindByEmail
	byEmail, err := userRepo.FindByEmail(ctx, "ALEX@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if byEmail.ID() != "u-1" {
		t.Fatalf("expected ID u-1, got %v", byEmail.ID())
	}

	// FindByUsername
	byName, err := userRepo.FindByUsername(ctx, "ALEX")
	if err != nil {
		t.Fatalf("FindByUsername: %v", err)
	}
	if byName.ID() != "u-1" {
		t.Fatalf("expected ID u-1, got %v", byName.ID())
	}

	// CountUsers
	count, err := userRepo.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}
}

func TestCredentialRepository_CRUD(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	credRepo := postgres.NewCredentialRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	user, _ := domain.NewUser("u-1", "alex", "alex@example.com", domain.RoleAdmin, now, now)
	_ = userRepo.Save(ctx, user)

	creds, _ := domain.NewUserCredentials("u-1", "$argon2id$v=19$...", now)
	if err := credRepo.Save(ctx, creds); err != nil {
		t.Fatalf("Save creds: %v", err)
	}

	got, err := credRepo.FindByUserID(ctx, "u-1")
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if got.PasswordHash() != "$argon2id$v=19$..." {
		t.Fatalf("expected hash match, got %s", got.PasswordHash())
	}
}

func TestRefreshTokenRepository_Lifecycle(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	rtRepo := postgres.NewRefreshTokenRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	user, _ := domain.NewUser("u-1", "alex", "alex@example.com", domain.RoleAdmin, now, now)
	_ = userRepo.Save(ctx, user)

	rt, _ := domain.NewRefreshToken("rt-1", "u-1", "hash123", now.Add(24*time.Hour), now)
	if err := rtRepo.Save(ctx, rt); err != nil {
		t.Fatalf("Save refresh token: %v", err)
	}

	got, err := rtRepo.FindByTokenHash(ctx, "hash123")
	if err != nil {
		t.Fatalf("FindByTokenHash: %v", err)
	}
	if got.ID() != "rt-1" || !got.IsActive(now) {
		t.Fatalf("expected active token rt-1, got %+v", got)
	}

	// RevokeAllForUser
	later := now.Add(time.Hour)
	if err := rtRepo.RevokeAllForUser(ctx, "u-1", later); err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}

	gotRevoked, err := rtRepo.FindByTokenHash(ctx, "hash123")
	if err != nil {
		t.Fatalf("FindByTokenHash after revoke: %v", err)
	}
	if gotRevoked.IsActive(now) {
		t.Fatal("expected token to be inactive after RevokeAllForUser")
	}
}

func TestMFARepository_CRUD(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	mfaRepo := postgres.NewMFARepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	user, _ := domain.NewUser("u-1", "alex", "alex@example.com", domain.RoleAdmin, now, now)
	_ = userRepo.Save(ctx, user)

	totp, _ := domain.NewTOTPSettings("u-1", []byte("encrypted-secret"), []string{"hash1", "hash2"}, true, &now, now)
	if err := mfaRepo.Save(ctx, totp); err != nil {
		t.Fatalf("Save MFA: %v", err)
	}

	got, err := mfaRepo.FindByUserID(ctx, "u-1")
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if !got.IsEnabled() || string(got.EncryptedSecret()) != "encrypted-secret" {
		t.Fatalf("got = %+v", got)
	}

	if err := mfaRepo.Delete(ctx, "u-1"); err != nil {
		t.Fatalf("Delete MFA: %v", err)
	}

	_, err = mfaRepo.FindByUserID(ctx, "u-1")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("category after delete = %v, want NotFound", domain.CategoryOf(err))
	}
}
