package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestLibraryAggregate(t *testing.T) {
	now := time.Now()
	libID := domain.LibraryID("lib-123")

	t.Run("valid library creation", func(t *testing.T) {
		lib, err := domain.NewLibrary(libID, "Personal Library", "My personal books", false, now, now)
		if err != nil {
			t.Fatalf("expected valid library, got: %v", err)
		}
		if lib.ID() != libID {
			t.Errorf("expected ID %v, got %v", libID, lib.ID())
		}
		if lib.Name() != "Personal Library" {
			t.Errorf("expected name 'Personal Library', got %v", lib.Name())
		}
		if lib.AllowReaderUploads() {
			t.Error("expected allowReaderUploads false")
		}

		lib.SetAllowReaderUploads(true, now.Add(time.Minute))
		if !lib.AllowReaderUploads() {
			t.Error("expected allowReaderUploads true after toggle")
		}
	})

	t.Run("empty or invalid name fails", func(t *testing.T) {
		_, err := domain.NewLibrary(libID, "", "desc", false, now, now)
		if err == nil {
			t.Error("expected error for empty name, got nil")
		}

		_, err = domain.NewLibrary(libID, "   ", "desc", false, now, now)
		if err == nil {
			t.Error("expected error for whitespace name, got nil")
		}
	})

	t.Run("rename updates name and updatedAt", func(t *testing.T) {
		lib, _ := domain.NewLibrary(libID, "Old Name", "desc", false, now, now)
		later := now.Add(10 * time.Minute)
		err := lib.Rename("New Name", later)
		if err != nil {
			t.Fatalf("unexpected rename error: %v", err)
		}
		if lib.Name() != "New Name" {
			t.Errorf("expected 'New Name', got %v", lib.Name())
		}
		if !lib.UpdatedAt().Equal(later) {
			t.Errorf("expected updatedAt %v, got %v", later, lib.UpdatedAt())
		}
	})
}

func TestLibraryMembership(t *testing.T) {
	now := time.Now()
	memID := domain.LibraryMembershipID("mem-1")
	libID := domain.LibraryID("lib-1")
	userID := domain.UserID("u-1")

	t.Run("valid membership", func(t *testing.T) {
		mem, err := domain.NewLibraryMembership(memID, libID, userID, domain.RoleReader, now)
		if err != nil {
			t.Fatalf("expected valid membership, got: %v", err)
		}
		if mem.Role() != domain.RoleReader {
			t.Errorf("expected reader role, got %v", mem.Role())
		}

		err = mem.SetRole(domain.RoleAdmin)
		if err != nil {
			t.Fatalf("unexpected set role error: %v", err)
		}
		if mem.Role() != domain.RoleAdmin {
			t.Errorf("expected admin role, got %v", mem.Role())
		}
	})

	t.Run("invalid IDs or role fails", func(t *testing.T) {
		_, err := domain.NewLibraryMembership("", libID, userID, domain.RoleReader, now)
		if err == nil {
			t.Error("expected error for empty ID, got nil")
		}
		_, err = domain.NewLibraryMembership(memID, "", userID, domain.RoleReader, now)
		if err == nil {
			t.Error("expected error for empty library ID, got nil")
		}
		_, err = domain.NewLibraryMembership(memID, libID, "", domain.RoleReader, now)
		if err == nil {
			t.Error("expected error for empty user ID, got nil")
		}
		_, err = domain.NewLibraryMembership(memID, libID, userID, domain.Role("bogus"), now)
		if err == nil {
			t.Error("expected error for bogus role, got nil")
		}
	})
}

func TestLibraryInvitation(t *testing.T) {
	now := time.Now()
	invID := domain.LibraryInvitationID("inv-1")
	libID := domain.LibraryID("lib-1")
	creatorID := domain.UserID("u-admin")
	expiresAt := now.Add(7 * 24 * time.Hour)

	inv, err := domain.NewLibraryInvitation(invID, libID, "invitee@example.com", domain.RoleReader, "hash123", creatorID, expiresAt, now)
	if err != nil {
		t.Fatalf("expected valid invitation, got: %v", err)
	}

	if !inv.IsValid(now) {
		t.Error("expected invitation to be valid now")
	}

	if inv.IsValid(now.Add(8 * 24 * time.Hour)) {
		t.Error("expected expired invitation to be invalid")
	}

	inv.MarkUsed(now.Add(time.Hour))
	if inv.IsValid(now) {
		t.Error("expected used invitation to be invalid")
	}
	if inv.UsedAt() == nil {
		t.Error("expected UsedAt to be non-nil")
	}
}
