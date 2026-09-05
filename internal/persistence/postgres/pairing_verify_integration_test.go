//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

type failingDeviceRepo struct {
	postgres.PairedDeviceRepository
	failInsert bool
}

func (f *failingDeviceRepo) InsertProvisional(
	ctx context.Context,
	id domain.DeviceID,
	label string,
	deviceClass domain.DeviceClass,
	enrolledVia domain.EnrolledVia,
	sessionID domain.PairingSessionID,
	now time.Time,
) error {
	if f.failInsert {
		return errors.New("simulated failure after session consumption")
	}
	return f.PairedDeviceRepository.InsertProvisional(ctx, id, label, deviceClass, enrolledVia, sessionID, now)
}

func setupVerifier(t *testing.T) (*postgres.PairingVerifier, *postgres.PairingSessionRepository, *postgres.PairedDeviceRepository, []byte, []byte) {
	pool := schemaTestPool(t)
	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	encKey, indexKey := testKeys()
	sessionRepo, err := postgres.NewPairingSessionRepository(pool, encKey, indexKey)
	if err != nil {
		t.Fatalf("NewPairingSessionRepository: %v", err)
	}
	deviceRepo := postgres.NewPairedDeviceRepository(pool)
	transactor := postgres.NewTransactor(pool)

	verifier := postgres.NewPairingVerifier(transactor, sessionRepo, deviceRepo)
	return verifier, sessionRepo, deviceRepo, encKey, indexKey
}

func TestPairingVerifier_VerifyAtomicSuccess(t *testing.T) {
	verifier, sessionRepo, deviceRepo, _, _ := setupVerifier(t)
	ctx := context.Background()

	code, _ := domain.NewPairingCode("2345-6789")
	now := time.Now().UTC().Truncate(time.Microsecond)
	session, _ := domain.NewPairingSession("ps-atomic-1", "user-admin-1", code, 5*time.Minute, now)

	if err := sessionRepo.Save(ctx, session); err != nil {
		t.Fatalf("sessionRepo.Save: %v", err)
	}

	devID := domain.DeviceID("dev-atomic-1")
	verified, err := verifier.VerifyAndConsume(
		ctx,
		code,
		devID,
		"Atomic Device",
		domain.DeviceClassDesktop,
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("VerifyAndConsume: %v", err)
	}

	if verified.State() != domain.PairingConsumed {
		t.Fatalf("verified state = %q, want consumed", verified.State())
	}
	if verified.DeviceID() == nil || *verified.DeviceID() != devID {
		t.Fatalf("verified deviceID = %v, want %v", verified.DeviceID(), devID)
	}

	// Verify device exists and is provisional with pairing_session_id set
	db := testDB(t)
	var storedSessionID, storedLabel string
	var storedOwner *string
	err = db.QueryRowContext(ctx,
		`SELECT label, pairing_session_id, owner_id FROM paired_devices WHERE id = $1`, string(devID)).
		Scan(&storedLabel, &storedSessionID, &storedOwner)
	if err != nil {
		t.Fatalf("query paired_devices: %v", err)
	}
	if storedLabel != "Atomic Device" || storedSessionID != "ps-atomic-1" || storedOwner != nil {
		t.Fatalf("paired_device row unexpected: label=%q sid=%q owner=%v", storedLabel, storedSessionID, storedOwner)
	}

	// Read session from independent connection: must be consumed
	reloaded, err := sessionRepo.FindByID(ctx, "ps-atomic-1")
	if err != nil {
		t.Fatalf("sessionRepo.FindByID: %v", err)
	}
	if reloaded.State() != domain.PairingConsumed {
		t.Fatalf("reloaded state = %q, want consumed", reloaded.State())
	}
	_ = deviceRepo
}

func TestPairingVerifier_MidTransactionFailureRollsBack(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	encKey, indexKey := testKeys()
	sessionRepo, err := postgres.NewPairingSessionRepository(pool, encKey, indexKey)
	if err != nil {
		t.Fatalf("NewPairingSessionRepository: %v", err)
	}

	baseDeviceRepo := postgres.NewPairedDeviceRepository(pool)
	failingDeviceRepo := &failingDeviceRepo{
		PairedDeviceRepository: *baseDeviceRepo,
		failInsert:             true,
	}
	transactor := postgres.NewTransactor(pool)

	verifier := postgres.NewPairingVerifierWithDeviceRepo(transactor, sessionRepo, failingDeviceRepo)

	code, _ := domain.NewPairingCode("3456-789A")
	now := time.Now().UTC().Truncate(time.Microsecond)
	session, _ := domain.NewPairingSession("ps-rollback-1", "user-admin-1", code, 5*time.Minute, now)
	if err := sessionRepo.Save(ctx, session); err != nil {
		t.Fatalf("sessionRepo.Save: %v", err)
	}

	devID := domain.DeviceID("dev-rollback-1")
	_, err = verifier.VerifyAndConsume(
		ctx,
		code,
		devID,
		"Rollback Device",
		domain.DeviceClassDesktop,
		now.Add(time.Minute),
	)
	if err == nil {
		t.Fatal("expected failure from failing device repo, got nil")
	}

	// Verify rollback from independent DB connection:
	// 1. Session MUST still be in pending state, device_id MUST be nil
	reloaded, err := sessionRepo.FindByID(ctx, "ps-rollback-1")
	if err != nil {
		t.Fatalf("FindByID after rollback: %v", err)
	}
	if reloaded.State() != domain.PairingPending {
		t.Fatalf("session state after rollback = %q, want pending", reloaded.State())
	}
	if reloaded.DeviceID() != nil {
		t.Fatalf("session deviceID after rollback = %v, want nil", reloaded.DeviceID())
	}

	// 2. Device MUST NOT exist
	db := testDB(t)
	var count int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM paired_devices WHERE id = $1`, string(devID)).Scan(&count)
	if err != nil {
		t.Fatalf("count paired_devices: %v", err)
	}
	if count != 0 {
		t.Fatalf("paired_devices count = %d, want 0 (transaction should have rolled back)", count)
	}
}

func TestPairingVerifier_ConcurrentDoubleSubmitBindsExactlyOne(t *testing.T) {
	verifier, sessionRepo, _, _, _ := setupVerifier(t)
	ctx := context.Background()

	code, _ := domain.NewPairingCode("5678-9ABC")
	now := time.Now().UTC().Truncate(time.Microsecond)
	session, _ := domain.NewPairingSession("ps-concurrent-1", "user-admin-1", code, 5*time.Minute, now)
	if err := sessionRepo.Save(ctx, session); err != nil {
		t.Fatalf("sessionRepo.Save: %v", err)
	}

	startGate := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var successCount int32
	var notFoundCount int32
	var otherErrors int32

	runAttempt := func(deviceID domain.DeviceID, label string) {
		defer wg.Done()
		<-startGate

		_, err := verifier.VerifyAndConsume(
			ctx,
			code,
			deviceID,
			label,
			domain.DeviceClassPhone,
			now.Add(time.Minute),
		)
		if err == nil {
			atomic.AddInt32(&successCount, 1)
		} else if domain.CategoryOf(err) == domain.NotFound {
			atomic.AddInt32(&notFoundCount, 1)
		} else {
			atomic.AddInt32(&otherErrors, 1)
			t.Logf("unexpected error in concurrent attempt: %v", err)
		}
	}

	go runAttempt("dev-race-1", "Device 1")
	go runAttempt("dev-race-2", "Device 2")

	close(startGate)
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("successCount = %d, want exactly 1", successCount)
	}
	if notFoundCount != 1 {
		t.Fatalf("notFoundCount = %d, want exactly 1", notFoundCount)
	}
	if otherErrors != 0 {
		t.Fatalf("otherErrors = %d, want 0", otherErrors)
	}

	// Verify in database: exactly 1 device row associated with ps-concurrent-1
	db := testDB(t)
	var deviceCount int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM paired_devices WHERE pairing_session_id = 'ps-concurrent-1'`).
		Scan(&deviceCount)
	if err != nil {
		t.Fatalf("query paired_devices count: %v", err)
	}
	if deviceCount != 1 {
		t.Fatalf("deviceCount in DB = %d, want exactly 1", deviceCount)
	}

	// Verify session in DB: state is consumed
	reloaded, err := sessionRepo.FindByID(ctx, "ps-concurrent-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if reloaded.State() != domain.PairingConsumed {
		t.Fatalf("session state = %q, want consumed", reloaded.State())
	}
}
