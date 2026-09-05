//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func testKeys() ([]byte, []byte) {
	encKey := bytes.Repeat([]byte{0x42}, 32)
	indexKey := bytes.Repeat([]byte{0x24}, 32)
	return encKey, indexKey
}

func seedTestAdmin(t *testing.T, pool interface{ Exec(context.Context, string, ...any) (any, error) }) {
	// Handled by mustExecPool in postgres_test
}

func TestPairingSessionRepository_SaveAndFindRoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	encKey, indexKey := testKeys()
	repo, err := postgres.NewPairingSessionRepository(pool, encKey, indexKey)
	if err != nil {
		t.Fatalf("NewPairingSessionRepository: %v", err)
	}

	rawCode := "3XYZ-4ABC"
	code, err := domain.NewPairingCode(rawCode)
	if err != nil {
		t.Fatalf("NewPairingCode: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	session, err := domain.NewPairingSession(
		domain.PairingSessionID("ps-test-1"),
		domain.UserID("user-admin-1"),
		code,
		5*time.Minute,
		now,
	)
	if err != nil {
		t.Fatalf("NewPairingSession: %v", err)
	}

	// 1. Save with initiator IP
	if err := repo.SaveWithInitiatorIP(ctx, session, "192.168.1.50"); err != nil {
		t.Fatalf("SaveWithInitiatorIP: %v", err)
	}

	// 2. Direct DB inspection: verify code is encrypted at rest and HMAC indexed
	var ciphertext, index []byte
	var initiatorIP string
	err = pool.QueryRow(ctx,
		`SELECT code_ciphertext, code_index, host(initiator_ip) FROM pairing_sessions WHERE id = 'ps-test-1'`).
		Scan(&ciphertext, &index, &initiatorIP)
	if err != nil {
		t.Fatalf("query raw pairing_session: %v", err)
	}

	// Assert ciphertext is NOT the plaintext code
	if bytes.Contains(ciphertext, []byte(code.Normalized())) {
		t.Fatal("code_ciphertext contains raw normalized pairing code! Must be encrypted.")
	}
	// Assert index matches expected HMAC-SHA256
	h := hmac.New(sha256.New, indexKey)
	h.Write([]byte(code.Normalized()))
	expectedIndex := h.Sum(nil)
	if !bytes.Equal(index, expectedIndex) {
		t.Fatalf("code_index mismatch: got %x, want %x", index, expectedIndex)
	}
	if initiatorIP != "192.168.1.50" {
		t.Fatalf("initiator_ip mismatch: got %q, want 192.168.1.50", initiatorIP)
	}

	// 3. FindByID decrypts and rehydrates
	loaded, err := repo.FindByID(ctx, domain.PairingSessionID("ps-test-1"))
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if loaded.ID() != session.ID() {
		t.Fatalf("loaded ID = %q, want %q", loaded.ID(), session.ID())
	}
	if loaded.InitiatedBy() != session.InitiatedBy() {
		t.Fatalf("loaded InitiatedBy = %q, want %q", loaded.InitiatedBy(), session.InitiatedBy())
	}
	if !loaded.Code().Equal(session.Code()) {
		t.Fatalf("loaded Code %q did not match original code %q", loaded.Code().Display(), session.Code().Display())
	}
	if loaded.State() != domain.PairingPending {
		t.Fatalf("loaded State = %q, want %q", loaded.State(), domain.PairingPending)
	}

	// 4. FindByCodeIndex
	byIndex, err := repo.FindByCodeIndex(ctx, repo.CodeIndex(code))
	if err != nil {
		t.Fatalf("FindByCodeIndex: %v", err)
	}
	if byIndex.ID() != session.ID() {
		t.Fatalf("byIndex ID = %q, want %q", byIndex.ID(), session.ID())
	}

	// 5. Update state and save
	devID := domain.DeviceID("dev-1")
	if err := session.Verify(now.Add(time.Minute), code, devID); err != nil {
		t.Fatalf("session.Verify: %v", err)
	}
	if err := repo.Save(ctx, session); err != nil {
		t.Fatalf("Save (update): %v", err)
	}

	updated, err := repo.FindByID(ctx, domain.PairingSessionID("ps-test-1"))
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}
	if updated.State() != domain.PairingVerified {
		t.Fatalf("updated State = %q, want %q", updated.State(), domain.PairingVerified)
	}
	if updated.DeviceID() == nil || *updated.DeviceID() != devID {
		t.Fatalf("updated DeviceID = %v, want %v", updated.DeviceID(), devID)
	}
}

func TestPairingSessionRepository_FindPendingByCodeIndexForUpdate(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	encKey, indexKey := testKeys()
	repo, err := postgres.NewPairingSessionRepository(pool, encKey, indexKey)
	if err != nil {
		t.Fatalf("NewPairingSessionRepository: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	code, _ := domain.NewPairingCode("ABC2-3456")
	session, _ := domain.NewPairingSession("ps-lock-1", "user-admin-1", code, 5*time.Minute, now)

	if err := repo.Save(ctx, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	idx := repo.CodeIndex(code)

	// Non-transactional context fails with domain.Internal
	_, err = repo.FindPendingByCodeIndexForUpdate(ctx, idx, now.Add(time.Minute))
	if err == nil || domain.CategoryOf(err) != domain.Internal {
		t.Fatalf("expected Internal error when called outside transaction, got: %v", err)
	}

	transactor := postgres.NewTransactor(pool)

	// Case 1: unexpired pending inside tx -> found
	err = transactor.InTx(ctx, func(txCtx context.Context) error {
		locked, err := repo.FindPendingByCodeIndexForUpdate(txCtx, idx, now.Add(time.Minute))
		if err != nil {
			return err
		}
		if locked.ID() != session.ID() {
			t.Fatalf("locked ID = %q, want %q", locked.ID(), session.ID())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTx case 1: %v", err)
	}

	// Case 2: past expiry inside tx -> NotFound
	err = transactor.InTx(ctx, func(txCtx context.Context) error {
		_, err := repo.FindPendingByCodeIndexForUpdate(txCtx, idx, now.Add(6*time.Minute))
		if err == nil || domain.CategoryOf(err) != domain.NotFound {
			t.Fatalf("expected NotFound for past expiry, got: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTx case 2: %v", err)
	}

	// Case 3: consumed session inside tx -> NotFound
	devID := domain.DeviceID("dev-1")
	_ = session.Verify(now.Add(time.Minute), code, devID)
	_ = session.Consume(now.Add(2 * time.Minute))
	if err := repo.Save(ctx, session); err != nil {
		t.Fatalf("Save consumed: %v", err)
	}

	err = transactor.InTx(ctx, func(txCtx context.Context) error {
		_, err := repo.FindPendingByCodeIndexForUpdate(txCtx, idx, now.Add(3*time.Minute))
		if err == nil || domain.CategoryOf(err) != domain.NotFound {
			t.Fatalf("expected NotFound for consumed session, got: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTx case 3: %v", err)
	}
}

func TestPairingSessionRepository_TamperedCiphertextFails(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	encKey, indexKey := testKeys()
	repo, err := postgres.NewPairingSessionRepository(pool, encKey, indexKey)
	if err != nil {
		t.Fatalf("NewPairingSessionRepository: %v", err)
	}

	code, _ := domain.NewPairingCode("WXYZ-8901")
	now := time.Now().UTC().Truncate(time.Microsecond)
	session, _ := domain.NewPairingSession("ps-tamper-1", "user-admin-1", code, 5*time.Minute, now)

	if err := repo.Save(ctx, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Corrupt ciphertext in DB
	if _, err := pool.Exec(ctx, `UPDATE pairing_sessions SET code_ciphertext = $1 WHERE id = $2`, []byte{0x00, 0x01, 0x02, 0x03}, "ps-tamper-1"); err != nil {
		t.Fatalf("corrupt ciphertext: %v", err)
	}

	_, err = repo.FindByID(ctx, "ps-tamper-1")
	if err == nil {
		t.Fatal("expected error on tampered ciphertext, got nil")
	}
	var domErr *domain.Error
	if !errors.As(err, &domErr) || domErr.Category != domain.Internal {
		t.Fatalf("expected Internal domain error on decrypt failure, got: %v", err)
	}
}

func TestPairingSessionRepository_Delete(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-del', 'admindel', 'admindel@example.com', 'admin', now(), now())`)

	encKey, indexKey := testKeys()
	repo, err := postgres.NewPairingSessionRepository(pool, encKey, indexKey)
	if err != nil {
		t.Fatalf("NewPairingSessionRepository: %v", err)
	}

	code, _ := domain.NewPairingCode("DEL1-2345")
	now := time.Now().UTC().Truncate(time.Microsecond)
	session, _ := domain.NewPairingSession("ps-del-1", "user-admin-del", code, 5*time.Minute, now)

	if err := repo.Save(ctx, session); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// First delete succeeds
	if err := repo.Delete(ctx, session.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// FindByID returns NotFound
	_, err = repo.FindByID(ctx, session.ID())
	if err == nil || domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("expected NotFound after delete, got: %v", err)
	}

	// Subsequent delete returns NotFound
	err = repo.Delete(ctx, session.ID())
	if err == nil || domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("expected NotFound on repeated delete, got: %v", err)
	}
}

func TestPairingSessionRepository_KeyValidation(t *testing.T) {
	pool := schemaTestPool(t)
	validKey := make([]byte, 32)
	shortKey := make([]byte, 16)

	if _, err := postgres.NewPairingSessionRepository(pool, shortKey, validKey); err == nil {
		t.Fatal("expected error for short encKey, got nil")
	}

	if _, err := postgres.NewPairingSessionRepository(pool, validKey, shortKey); err == nil {
		t.Fatal("expected error for short indexKey, got nil")
	}
}
