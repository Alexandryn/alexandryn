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
