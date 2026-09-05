//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestPairedDeviceRepository_CRUD(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-dev-1', 'devuser1', 'devuser1@example.com', 'reader', now(), now())`)

	devRepo := postgres.NewPairedDeviceRepository(pool)

	now := time.Now().UTC().Truncate(time.Microsecond)
	devID := domain.DeviceID("dev-phone-1")
	device, err := domain.NewPairedDevice(
		devID,
		domain.UserID("user-dev-1"),
		"Pixel 8",
		domain.DeviceClassPhone,
		domain.EnrolledViaPairingCode,
		now,
	)
	if err != nil {
		t.Fatalf("NewPairedDevice: %v", err)
	}

	// 1. Save
	if err := devRepo.Save(ctx, device); err != nil {
		t.Fatalf("devRepo.Save: %v", err)
	}

	// 2. FindByID
	loaded, err := devRepo.FindByID(ctx, devID)
	if err != nil {
		t.Fatalf("devRepo.FindByID: %v", err)
	}
	if loaded.ID() != devID || loaded.Owner() != domain.UserID("user-dev-1") || loaded.Label() != "Pixel 8" {
		t.Fatalf("loaded unexpected: id=%q owner=%q label=%q", loaded.ID(), loaded.Owner(), loaded.Label())
	}
	if loaded.DeviceClass() != domain.DeviceClassPhone || loaded.EnrolledVia() != domain.EnrolledViaPairingCode {
		t.Fatalf("loaded unexpected class/via: class=%q via=%q", loaded.DeviceClass(), loaded.EnrolledVia())
	}

	// 3. FindByOwner
	byOwner, err := devRepo.FindByOwner(ctx, domain.UserID("user-dev-1"))
	if err != nil {
		t.Fatalf("devRepo.FindByOwner: %v", err)
	}
	if len(byOwner) != 1 || byOwner[0].ID() != devID {
		t.Fatalf("expected 1 device, got %d", len(byOwner))
	}

	// 4. Revoke
	revokeTime := now.Add(10 * time.Minute)
	if err := devRepo.Revoke(ctx, devID, revokeTime); err != nil {
		t.Fatalf("devRepo.Revoke: %v", err)
	}
	revoked, err := devRepo.FindByID(ctx, devID)
	if err != nil {
		t.Fatalf("devRepo.FindByID after revoke: %v", err)
	}
	if revoked.RevokedAt() == nil {
		t.Fatal("expected RevokedAt to be set")
	}

	// 5. Provisional insertion and AssignOwnerByPairingSession
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-prov-1', 'user-dev-1', '\x01', '\x02', 'consumed', now(), now() + interval '5 minutes')`)

	provID := domain.DeviceID("dev-provisional-1")
	err = devRepo.InsertProvisional(
		ctx,
		provID,
		"Provisional Tablet",
		domain.DeviceClassTablet,
		domain.EnrolledViaPairingCode,
		domain.PairingSessionID("ps-prov-1"),
		now,
	)
	if err != nil {
		t.Fatalf("InsertProvisional: %v", err)
	}

	// FindByPairingSessionID on provisional device returns NotFound (unassigned owner)
	_, err = devRepo.FindByPairingSessionID(ctx, domain.PairingSessionID("ps-prov-1"))
	if err == nil || domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("expected NotFound for provisional device FindByPairingSessionID, got: %v", err)
	}

	// Assign owner
	err = devRepo.AssignOwnerByPairingSession(ctx, domain.PairingSessionID("ps-prov-1"), domain.UserID("user-dev-1"))
	if err != nil {
		t.Fatalf("AssignOwnerByPairingSession: %v", err)
	}

	assigned, err := devRepo.FindByID(ctx, provID)
	if err != nil {
		t.Fatalf("FindByID assigned device: %v", err)
	}
	if assigned.Owner() != domain.UserID("user-dev-1") {
		t.Fatalf("assigned owner = %q, want user-dev-1", assigned.Owner())
	}

	// FindByPairingSessionID on claimed device succeeds
	bySession, err := devRepo.FindByPairingSessionID(ctx, domain.PairingSessionID("ps-prov-1"))
	if err != nil {
		t.Fatalf("FindByPairingSessionID after assign: %v", err)
	}
	if bySession.ID() != provID || bySession.Owner() != domain.UserID("user-dev-1") {
		t.Fatalf("bySession unexpected: id=%q owner=%q", bySession.ID(), bySession.Owner())
	}

	// Non-existent session returns NotFound
	_, err = devRepo.FindByPairingSessionID(ctx, domain.PairingSessionID("ps-nonexistent"))
	if err == nil || domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("expected NotFound for non-existent session FindByPairingSessionID, got: %v", err)
	}
}

// TestPairedDeviceRepository_RevokeByPairingSessionID reproduces the bug
// where an admin's revoke of a still-provisional device (verified but not
// yet claimed by login, owner_id NULL) silently did nothing: the old code
// path went through FindByPairingSessionID -> Revoke(id), and
// FindByPairingSessionID returns NotFound for a provisional row (Owner is a
// mandatory domain.PairedDevice field, so scanDevice can't represent one).
// RevokeByPairingSessionID revokes by pairing_session_id directly, so it
// works regardless of whether the device has been claimed yet.
func TestPairedDeviceRepository_RevokeByPairingSessionID(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-dev-2', 'devuser2', 'devuser2@example.com', 'reader', now(), now())`)

	devRepo := postgres.NewPairedDeviceRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-revoke-prov', 'user-dev-2', '\x01', '\x03', 'consumed', now(), now() + interval '5 minutes')`)

	provID := domain.DeviceID("dev-revoke-provisional")
	if err := devRepo.InsertProvisional(
		ctx, provID, "Provisional Phone", domain.DeviceClassPhone, domain.EnrolledViaPairingCode,
		domain.PairingSessionID("ps-revoke-prov"), now,
	); err != nil {
		t.Fatalf("InsertProvisional: %v", err)
	}

	// The bug: revoking through the device (never assigned) is a no-op —
	// there is no device ID to revoke by until ownership is assigned.
	// The fix under test revokes by session ID instead.
	revokeTime := now.Add(time.Minute)
	if err := devRepo.RevokeByPairingSessionID(ctx, domain.PairingSessionID("ps-revoke-prov"), revokeTime); err != nil {
		t.Fatalf("RevokeByPairingSessionID on provisional device: %v", err)
	}

	// A revoked provisional device must not become claimable by a later
	// login completing with the same grant (closes the "admin revokes,
	// device still gets fully paired" gap).
	err := devRepo.AssignOwnerByPairingSession(ctx, domain.PairingSessionID("ps-revoke-prov"), domain.UserID("user-dev-2"))
	if err == nil || domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("expected AssignOwnerByPairingSession to fail (NotFound) for a revoked provisional device, got: %v", err)
	}

	// Revoking an already-revoked (or nonexistent) session is reported, not silently ok.
	if err := devRepo.RevokeByPairingSessionID(ctx, domain.PairingSessionID("ps-revoke-prov"), revokeTime); err == nil || domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("expected NotFound revoking an already-revoked session, got: %v", err)
	}

	// RevokeByPairingSessionID also works for an already-owned device.
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-revoke-owned', 'user-dev-2', '\x01', '\x04', 'consumed', now(), now() + interval '5 minutes')`)
	ownedID := domain.DeviceID("dev-revoke-owned")
	if err := devRepo.InsertProvisional(
		ctx, ownedID, "Owned Phone", domain.DeviceClassPhone, domain.EnrolledViaPairingCode,
		domain.PairingSessionID("ps-revoke-owned"), now,
	); err != nil {
		t.Fatalf("InsertProvisional (owned): %v", err)
	}
	if err := devRepo.AssignOwnerByPairingSession(ctx, domain.PairingSessionID("ps-revoke-owned"), domain.UserID("user-dev-2")); err != nil {
		t.Fatalf("AssignOwnerByPairingSession (owned): %v", err)
	}
	if err := devRepo.RevokeByPairingSessionID(ctx, domain.PairingSessionID("ps-revoke-owned"), revokeTime); err != nil {
		t.Fatalf("RevokeByPairingSessionID on owned device: %v", err)
	}
	owned, err := devRepo.FindByID(ctx, ownedID)
	if err != nil {
		t.Fatalf("FindByID owned device: %v", err)
	}
	if owned.RevokedAt() == nil {
		t.Fatal("expected owned device RevokedAt to be set")
	}
}

func TestNetworkSettingsRepository(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	repo := postgres.NewNetworkSettingsRepository(pool)

	// Initial default row exists from migration
	initial, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("repo.Get initial: %v", err)
	}
	if initial.RememberDeviceDays != 30 {
		t.Fatalf("expected 30 days default, got %d", initial.RememberDeviceDays)
	}

	// Upsert
	now := time.Now().UTC().Truncate(time.Microsecond)
	settings := &domain.NetworkSettings{
		HostName:           "library.local",
		RememberDeviceDays: 60,
		UpdatedAt:          now,
	}
	if err := repo.Upsert(ctx, settings); err != nil {
		t.Fatalf("repo.Upsert: %v", err)
	}

	updated, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("repo.Get after upsert: %v", err)
	}
	if updated.HostName != "library.local" || updated.RememberDeviceDays != 60 {
		t.Fatalf("updated unexpected: host=%q days=%d", updated.HostName, updated.RememberDeviceDays)
	}
}

func TestEnrolmentGrantJTIRepository(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	repo := postgres.NewEnrolmentGrantJTIRepository(pool)

	jti := "test-jti-uuid-999"

	// Exists initially false
	exists, err := repo.Exists(ctx, jti)
	if err != nil {
		t.Fatalf("repo.Exists initial: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false for new jti")
	}

	// Record
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.Record(ctx, jti, now); err != nil {
		t.Fatalf("repo.Record: %v", err)
	}

	// Exists now true
	exists, err = repo.Exists(ctx, jti)
	if err != nil {
		t.Fatalf("repo.Exists after record: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true after record")
	}

	// Replay (duplicate Record) errors
	err = repo.Record(ctx, jti, now)
	if err == nil {
		t.Fatal("expected duplicate Record to fail, got nil")
	}
}
