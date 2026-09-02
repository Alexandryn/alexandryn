//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestLibraryRepository_CRUD(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	libRepo := postgres.NewLibraryRepository(pool)
	memRepo := postgres.NewLibraryMembershipRepository(pool)
	userRepo := postgres.NewUserRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	user, _ := domain.NewUser("u-admin", "alex", "alex@example.com", domain.RoleAdmin, now, now)
	_ = userRepo.Save(ctx, user)

	lib, err := domain.NewLibrary("lib-custom", "Comics", "Comic books collection", true, now, now)
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}

	// Save
	if err := libRepo.Save(ctx, lib); err != nil {
		t.Fatalf("Save library: %v", err)
	}

	// FindByID
	got, err := libRepo.FindByID(ctx, "lib-custom")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name() != "Comics" || !got.AllowReaderUploads() {
		t.Fatalf("got = %+v", got)
	}

	// Membership
	mem, _ := domain.NewLibraryMembership("mem-1", "lib-custom", "u-admin", domain.RoleAdmin, now)
	if err := memRepo.Save(ctx, mem); err != nil {
		t.Fatalf("Save membership: %v", err)
	}

	// FindMembership
	gotMem, err := memRepo.FindMembership(ctx, "lib-custom", "u-admin")
	if err != nil {
		t.Fatalf("FindMembership: %v", err)
	}
	if gotMem.Role() != domain.RoleAdmin {
		t.Fatalf("expected admin role, got %v", gotMem.Role())
	}

	// FindByUser
	userLibs, err := libRepo.FindByUser(ctx, "u-admin")
	if err != nil {
		t.Fatalf("FindByUser: %v", err)
	}
	if len(userLibs) != 1 || userLibs[0].ID() != "lib-custom" {
		t.Fatalf("expected 1 user lib, got %+v", userLibs)
	}

	// Delete
	if err := libRepo.Delete(ctx, "lib-custom"); err != nil {
		t.Fatalf("Delete library: %v", err)
	}

	_, err = libRepo.FindByID(ctx, "lib-custom")
	if domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("expected NotFound after delete, got %v", domain.CategoryOf(err))
	}
}

func TestLibraryInvitationRepository_Lifecycle(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	userRepo := postgres.NewUserRepository(pool)
	invRepo := postgres.NewLibraryInvitationRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	user, _ := domain.NewUser("u-admin", "alex", "alex@example.com", domain.RoleAdmin, now, now)
	_ = userRepo.Save(ctx, user)

	inv, err := domain.NewLibraryInvitation("inv-1", domain.DefaultLibraryID, "friend@example.com", domain.RoleReader, "hash-inv-1", "u-admin", now.Add(7*24*time.Hour), now)
	if err != nil {
		t.Fatalf("NewLibraryInvitation: %v", err)
	}

	if err := invRepo.Save(ctx, inv); err != nil {
		t.Fatalf("Save invitation: %v", err)
	}

	got, err := invRepo.FindByTokenHash(ctx, "hash-inv-1")
	if err != nil {
		t.Fatalf("FindByTokenHash: %v", err)
	}
	if got.Email() != "friend@example.com" || !got.IsValid(now) {
		t.Fatalf("got = %+v", got)
	}

	got.MarkUsed(now.Add(time.Hour))
	if err := invRepo.Save(ctx, got); err != nil {
		t.Fatalf("Save used invitation: %v", err)
	}

	gotAfter, err := invRepo.FindByTokenHash(ctx, "hash-inv-1")
	if err != nil {
		t.Fatalf("FindByTokenHash: %v", err)
	}
	if gotAfter.IsValid(now) {
		t.Fatal("expected used invitation to be invalid")
	}
}
