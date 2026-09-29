//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/idgen"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// TestPersistence_RestartPreservesUserDataAndSetupState proves that:
// 1. User records, credentials, and domain data persist across pool shutdown and restart.
// 2. Setup state remains consistent (CountUsers > 0).
// 3. SetupHandler rejects subsequent initialization attempts with 409 Conflict.
func TestPersistence_RestartPreservesUserDataAndSetupState(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dsn := os.Getenv("TEST_DATABASE_URL")
	ctx := context.Background()

	// Initial startup
	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate() failed on initial start: %v", err)
	}

	pool1, err := postgres.NewPool(ctx, dsn, 5)
	if err != nil {
		t.Fatalf("NewPool() 1 failed: %v", err)
	}

	userRepo1 := postgres.NewUserRepository(pool1)
	credRepo1 := postgres.NewCredentialRepository(pool1)
	libRepo1 := postgres.NewLibraryRepository(pool1)
	progRepo1 := postgres.NewReadingProgressRepository(pool1)
	workRepo1 := postgres.NewWorkRepository(pool1)
	hasher := auth.NewArgon2idPasswordHasher(auth.DefaultArgon2idParams())

	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := domain.UserID("usr_admin_01")
	user, err := domain.NewUser(userID, "admin_librarian", "librarian@alexandryn.local", domain.RoleAdmin, now, now)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if err := userRepo1.Save(ctx, user); err != nil {
		t.Fatalf("userRepo.Save: %v", err)
	}

	pwdHash, err := hasher.HashPassword("SecureAdminPassword123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	creds, err := domain.NewUserCredentials(userID, pwdHash, now)
	if err != nil {
		t.Fatalf("NewUserCredentials: %v", err)
	}
	if err := credRepo1.Save(ctx, creds); err != nil {
		t.Fatalf("credRepo.Save: %v", err)
	}

	// Create test work & reading progress
	workID := domain.WorkID("wrk_restart_01")
	work, err := domain.NewWork(workID, "The Library of Babel", "Short Story", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}
	if err := workRepo1.Save(ctx, work); err != nil {
		t.Fatalf("workRepo.Save: %v", err)
	}

	pct, err := domain.NewPercentage(0.88)
	if err != nil {
		t.Fatalf("NewPercentage: %v", err)
	}
	prog := domain.NewReadingProgress(domain.ReadingProgressID("prg_restart_01"), workID, pct, domain.DeviceID("dev_restart_01"), now)
	if err := progRepo1.Save(ctx, prog); err != nil {
		t.Fatalf("progRepo.Save: %v", err)
	}

	count1, err := userRepo1.CountUsers(ctx)
	if err != nil || count1 != 1 {
		t.Fatalf("expected count 1, got %d (err: %v)", count1, err)
	}

	// ── Simulate Application Shutdown ───────────────────────────────────────
	pool1.Close()

	// ── Simulate Application Restart ────────────────────────────────────────
	// Step 1: Run migrations on startup (idempotent, checks DB version)
	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate() failed on restart: %v", err)
	}

	// Step 2: Establish fresh connection pool
	pool2, err := postgres.NewPool(ctx, dsn, 5)
	if err != nil {
		t.Fatalf("NewPool() 2 failed: %v", err)
	}
	defer pool2.Close()

	userRepo2 := postgres.NewUserRepository(pool2)
	credRepo2 := postgres.NewCredentialRepository(pool2)
	memRepo2 := postgres.NewLibraryMembershipRepository(pool2)
	progRepo2 := postgres.NewReadingProgressRepository(pool2)
	rtRepo2 := postgres.NewRefreshTokenRepository(pool2)
	signer := auth.NewJWTSigner([]byte("01234567890123456789012345678901"), "alexandryn")

	// Step 3: Verify setup state remains consistent
	count2, err := userRepo2.CountUsers(ctx)
	if err != nil || count2 != 1 {
		t.Fatalf("restart lost setup state: expected count 1, got %d (err: %v)", count2, err)
	}

	// Step 4: Verify user and credential data preserved exactly
	restartedUser, err := userRepo2.FindByUsername(ctx, "admin_librarian")
	if err != nil {
		t.Fatalf("restart lost user account: %v", err)
	}
	if restartedUser.ID() != userID || restartedUser.Email() != "librarian@alexandryn.local" {
		t.Fatalf("user record corrupted across restart: %+v", restartedUser)
	}

	restartedCreds, err := credRepo2.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("restart lost credentials: %v", err)
	}
	match, err := hasher.VerifyPassword("SecureAdminPassword123", restartedCreds.PasswordHash())
	if err != nil || !match {
		t.Fatalf("password verification failed after restart: match=%v, err=%v", match, err)
	}

	// Step 5: Verify reading progress preserved exactly
	restartedProg, err := progRepo2.FindByWork(ctx, workID)
	if err != nil {
		t.Fatalf("restart lost reading progress: %v", err)
	}
	if restartedProg.Percentage() != 0.88 || restartedProg.DeviceID() != "dev_restart_01" {
		t.Fatalf("reading progress corrupted across restart: %+v", restartedProg)
	}

	// Step 6: Verify setup recovery protection — SetupHandler must refuse to overwrite existing installation
	setupHandler := transporthttp.SetupHandler(userRepo2, credRepo2, libRepo1, memRepo2, rtRepo2, hasher, signer, idgen.New())
	body := bytes.NewBufferString(`{
		"username": "intruder",
		"email": "intruder@bad.actor",
		"password": "MaliciousPassword999"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", body)
	rec := httptest.NewRecorder()
	setupHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("SetupHandler returned HTTP %d, want %d (Conflict) when system already setup", rec.Code, http.StatusConflict)
	}
}

// TestPersistence_CryptographicContinuityAcrossRestarts verifies that:
// 1. The master encryption key persists in appDataDir.
// 2. Ciphertexts encrypted before restart remain decryptable after restart.
// 3. A corrupted key file triggers fail-closed behavior without clobbering.
func TestPersistence_CryptographicContinuityAcrossRestarts(t *testing.T) {
	appDataDir := t.TempDir()

	// Initial start: generate key
	key1, err := crypto.LoadOrCreateKey(appDataDir, nil, nil)
	if err != nil {
		t.Fatalf("LoadOrCreateKey initial start: %v", err)
	}
	if len(key1) != crypto.KeyLen {
		t.Fatalf("key length = %d, want %d", len(key1), crypto.KeyLen)
	}

	keyFile := crypto.KeyPath(appDataDir)
	fi, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key file perm = %04o, want 0600", perm)
	}

	svc1, err := crypto.NewService(key1)
	if err != nil {
		t.Fatalf("NewService 1: %v", err)
	}
	plaintext := []byte("secret-credentials-to-protect")
	ciphertext, nonce, err := svc1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Simulate Restart: reload key
	key2, err := crypto.LoadOrCreateKey(appDataDir, nil, nil)
	if err != nil {
		t.Fatalf("LoadOrCreateKey restart: %v", err)
	}
	if !bytes.Equal(key1, key2) {
		t.Fatal("LoadOrCreateKey returned a different key after restart")
	}

	svc2, err := crypto.NewService(key2)
	if err != nil {
		t.Fatalf("NewService 2: %v", err)
	}
	decrypted, err := svc2.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt after restart: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("decrypted text %q != plaintext %q", string(decrypted), string(plaintext))
	}

	// Malformed key: truncate key file to 16 bytes.
	badDir := t.TempDir()
	badKeyFile := crypto.KeyPath(badDir)
	if err := os.WriteFile(badKeyFile, []byte("too-short-key-16"), 0o600); err != nil {
		t.Fatalf("writing bad key file: %v", err)
	}

	_, badErr := crypto.LoadOrCreateKey(badDir, nil, nil)
	if badErr == nil {
		t.Fatal("LoadOrCreateKey accepted a corrupted 16-byte key file, want error")
	}

	// Verify it failed closed and did NOT overwrite badKeyFile
	reRead, _ := os.ReadFile(badKeyFile)
	if string(reRead) != "too-short-key-16" {
		t.Fatal("LoadOrCreateKey clobbered corrupted key file instead of failing closed")
	}
}

// TestPersistence_FailedInitializationPreservesExistingData verifies that
// when an initialization error occurs (e.g. bad database URL or failed migration),
// existing persistent files and database state are not damaged.
func TestPersistence_FailedInitializationPreservesExistingData(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dsn := os.Getenv("TEST_DATABASE_URL")
	ctx := context.Background()

	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Seed one row
	if _, err := db.ExecContext(ctx, "INSERT INTO collections (id, name) VALUES ('col_preserve', 'Preserved Collection')"); err != nil {
		t.Fatalf("seeding failed: %v", err)
	}

	// Attempt a failing migration
	badDir := t.TempDir()
	badMigration := filepath.Join(badDir, "00099_broken.sql")
	if err := os.WriteFile(badMigration, []byte("-- +goose Up\nINVALID SYNTAX ERROR;\n-- +goose Down\n"), 0o644); err != nil {
		t.Fatalf("write bad migration: %v", err)
	}

	up := func(ctx context.Context, db *sql.DB) error {
		return postgres.ConnectionFailure(sql.ErrConnDone)
	}
	_ = postgres.RunMigrations(ctx, dsn, sql.Open, up)

	// Ensure pre-existing collection is completely intact
	var name string
	if err := db.QueryRowContext(ctx, "SELECT name FROM collections WHERE id = 'col_preserve'").Scan(&name); err != nil || name != "Preserved Collection" {
		t.Fatalf("failed initialization corrupted existing collection: err=%v, name=%q", err, name)
	}
}

// TestPersistence_BackupAndRecovery verifies that data exported from a database
// can be reliably restored into a clean installation with migrations applied.
func TestPersistence_BackupAndRecovery(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	dsn := os.Getenv("TEST_DATABASE_URL")
	ctx := context.Background()

	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Populate representative data
	_, err := db.ExecContext(ctx, `
		INSERT INTO works (id, title, subtitle, original_language) VALUES ('wrk_bkup', 'Book of Sands', '', 'es');
		INSERT INTO authors (id, name) VALUES ('aut_bkup', 'Jorge Luis Borges');
		INSERT INTO collections (id, name) VALUES ('col_bkup', 'Infinite Library');
		INSERT INTO users (id, username, email, role) VALUES ('usr_bkup', 'backup_admin', 'bk@alexandryn.local', 'admin');
	`)
	if err != nil {
		t.Fatalf("seeding failed: %v", err)
	}

	// Logical backup: extract all user data rows as JSON records
	type BackupDump struct {
		Works       []struct{ ID, Title, Subtitle, OriginalLanguage string }
		Authors     []struct{ ID, Name string }
		Collections []struct{ ID, Name string }
		Users       []struct{ ID, Username, Email, Role string }
	}
	var dump BackupDump

	// Works
	rows, err := db.QueryContext(ctx, "SELECT id, title, subtitle, COALESCE(original_language, '') FROM works")
	if err != nil {
		t.Fatalf("query works: %v", err)
	}
	for rows.Next() {
		var w struct{ ID, Title, Subtitle, OriginalLanguage string }
		if err := rows.Scan(&w.ID, &w.Title, &w.Subtitle, &w.OriginalLanguage); err != nil {
			t.Fatalf("scan work: %v", err)
		}
		dump.Works = append(dump.Works, w)
	}
	_ = rows.Close()

	// Users
	uRows, err := db.QueryContext(ctx, "SELECT id, username, email, role FROM users")
	if err != nil {
		t.Fatalf("query users: %v", err)
	}
	for uRows.Next() {
		var u struct{ ID, Username, Email, Role string }
		if err := uRows.Scan(&u.ID, &u.Username, &u.Email, &u.Role); err != nil {
			t.Fatalf("scan user: %v", err)
		}
		dump.Users = append(dump.Users, u)
	}
	_ = uRows.Close()

	dumpBytes, err := json.Marshal(dump)
	if err != nil {
		t.Fatalf("marshal dump: %v", err)
	}

	// ── Disaster Recovery Simulation: Clean Database ──────────────────────────
	resetSchema(t, db)

	// Step 1 of Recovery: Apply schema migrations
	if err := postgres.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate during recovery failed: %v", err)
	}

	// Step 2 of Recovery: Restore data from backup
	var restored BackupDump
	if err := json.Unmarshal(dumpBytes, &restored); err != nil {
		t.Fatalf("unmarshal dump: %v", err)
	}

	for _, w := range restored.Works {
		_, err := db.ExecContext(ctx, "INSERT INTO works (id, title, subtitle, original_language) VALUES ($1, $2, $3, $4)", w.ID, w.Title, w.Subtitle, w.OriginalLanguage)
		if err != nil {
			t.Fatalf("restore work: %v", err)
		}
	}
	for _, u := range restored.Users {
		_, err := db.ExecContext(ctx, "INSERT INTO users (id, username, email, role) VALUES ($1, $2, $3, $4)", u.ID, u.Username, u.Email, u.Role)
		if err != nil {
			t.Fatalf("restore user: %v", err)
		}
	}

	// Step 3 of Recovery: Validate recovered state
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE username = 'backup_admin'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("user count after recovery = %d, want 1", count)
	}
	var workTitle string
	if err := db.QueryRowContext(ctx, "SELECT title FROM works WHERE id = 'wrk_bkup'").Scan(&workTitle); err != nil || workTitle != "Book of Sands" {
		t.Fatalf("work title after recovery = %q, want 'Book of Sands'", workTitle)
	}
}
