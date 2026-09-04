//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// backend-network-api.md FR-7 / T3.1: migration 00010 creates the four network tables:
// pairing_sessions, paired_devices, enrolment_grant_jtis, network_settings.
func TestSchema_Phase13NetworkTablesExistAfterMigration(t *testing.T) {
	pool := schemaTestPool(t)

	// Seed user for foreign keys
	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	// 1. pairing_sessions round-trip
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at, device_id, initiator_ip)
		VALUES ('ps-1', 'user-admin-1', '\x01020304', '\x05060708', 'pending', now(), now() + interval '5 minutes', NULL, '192.168.1.50'::inet)`)

	var psState string
	var initiatorIP string
	err := pool.QueryRow(context.Background(),
		`SELECT state, host(initiator_ip) FROM pairing_sessions WHERE id = 'ps-1'`).Scan(&psState, &initiatorIP)
	if err != nil {
		t.Fatalf("scan pairing_sessions columns: %v", err)
	}
	if psState != "pending" || initiatorIP != "192.168.1.50" {
		t.Fatalf("pairing_sessions row unexpected: state=%q ip=%q", psState, initiatorIP)
	}

	// 2. paired_devices round-trip (with nullable owner_id and pairing_session_id)
	mustExecPool(t, pool, `INSERT INTO paired_devices
		(id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at, revoked_at, pairing_session_id)
		VALUES ('dev-1', 'user-admin-1', 'Phone', 'phone', 'pairing_code', now(), now(), NULL, 'ps-1')`)

	var devClass, enrolledVia string
	err = pool.QueryRow(context.Background(),
		`SELECT device_class, enrolled_via FROM paired_devices WHERE id = 'dev-1'`).Scan(&devClass, &enrolledVia)
	if err != nil {
		t.Fatalf("scan paired_devices columns: %v", err)
	}
	if devClass != "phone" || enrolledVia != "pairing_code" {
		t.Fatalf("paired_devices row unexpected: class=%q via=%q", devClass, enrolledVia)
	}

	// 3. enrolment_grant_jtis round-trip
	mustExecPool(t, pool, `INSERT INTO enrolment_grant_jtis (jti, spent_at)
		VALUES ('jti-abc-123', now())`)

	var jti string
	err = pool.QueryRow(context.Background(),
		`SELECT jti FROM enrolment_grant_jtis WHERE jti = 'jti-abc-123'`).Scan(&jti)
	if err != nil {
		t.Fatalf("scan enrolment_grant_jtis columns: %v", err)
	}
	if jti != "jti-abc-123" {
		t.Fatalf("enrolment_grant_jtis row unexpected: jti=%q", jti)
	}

	// 4. network_settings round-trip
	mustExecPool(t, pool, `INSERT INTO network_settings (id, host_name, remember_device_days, updated_at)
		VALUES ('default', 'alexandryn.local', 45, now())
		ON CONFLICT (id) DO UPDATE SET host_name = EXCLUDED.host_name, remember_device_days = EXCLUDED.remember_device_days`)

	var hostName string
	var rememberDays int
	err = pool.QueryRow(context.Background(),
		`SELECT host_name, remember_device_days FROM network_settings WHERE id = 'default'`).Scan(&hostName, &rememberDays)
	if err != nil {
		t.Fatalf("scan network_settings columns: %v", err)
	}
	if hostName != "alexandryn.local" || rememberDays != 45 {
		t.Fatalf("network_settings row unexpected: hostName=%q days=%d", hostName, rememberDays)
	}
}

// FR-7: pairing_sessions state is constrained to {pending, verified, consumed, expired};
// code_index is UNIQUE; (state, expires_at) index exists.
func TestSchema_PairingSessionsConstraints(t *testing.T) {
	pool := schemaTestPool(t)

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	// Invalid state check constraint
	_, err := pool.Exec(context.Background(), `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-bad', 'user-admin-1', '\x01', '\x02', 'invalid_state', now(), now() + interval '5 minutes')`)
	assertCheckViolation(t, err)

	// Valid states: pending, verified, consumed, expired
	for i, s := range []string{"pending", "verified", "consumed", "expired"} {
		id := "ps-valid-" + s
		codeIdx := []byte{byte(i + 10)}
		_, err := pool.Exec(context.Background(), `INSERT INTO pairing_sessions
			(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
			VALUES ($1, 'user-admin-1', '\x01', $2, $3, now(), now() + interval '5 minutes')`, id, codeIdx, s)
		if err != nil {
			t.Fatalf("insert valid state %q: %v", s, err)
		}
	}

	// Duplicate code_index unique violation
	_, err = pool.Exec(context.Background(), `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-dup', 'user-admin-1', '\x01', '\x0a', 'pending', now(), now() + interval '5 minutes')`)
	assertUniqueViolation(t, err)

	// Check index on (state, expires_at) exists
	var indexExists bool
	err = pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'public'
			  AND tablename = 'pairing_sessions'
			  AND indexdef LIKE '%(state, expires_at)%'
		)`).Scan(&indexExists)
	if err != nil {
		t.Fatalf("query index existence: %v", err)
	}
	if !indexExists {
		t.Fatal("expected index on (state, expires_at) on pairing_sessions")
	}
}

// FR-7: paired_devices constraints:
// - device_class ∈ {phone,tablet,desktop,tv,unknown}
// - enrolled_via ∈ {pairing_code,password_login}
// - owner_id is nullable (provisional pairing)
// - pairing_session_id ON DELETE SET NULL
func TestSchema_PairedDevicesConstraints(t *testing.T) {
	pool := schemaTestPool(t)

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-admin-1', 'admin1', 'admin1@example.com', 'admin', now(), now())`)

	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-ref', 'user-admin-1', '\x01', '\x02', 'pending', now(), now() + interval '5 minutes')`)

	// Invalid device_class
	_, err := pool.Exec(context.Background(), `INSERT INTO paired_devices
		(id, label, device_class, enrolled_via, created_at, last_seen_at)
		VALUES ('dev-bad-class', 'My Device', 'smartwatch', 'pairing_code', now(), now())`)
	assertCheckViolation(t, err)

	// Invalid enrolled_via
	_, err = pool.Exec(context.Background(), `INSERT INTO paired_devices
		(id, label, device_class, enrolled_via, created_at, last_seen_at)
		VALUES ('dev-bad-via', 'My Device', 'phone', 'magic_link', now(), now())`)
	assertCheckViolation(t, err)

	// Provisional device (owner_id IS NULL) is permitted
	mustExecPool(t, pool, `INSERT INTO paired_devices
		(id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at, pairing_session_id)
		VALUES ('dev-provisional', NULL, 'Living Room TV', 'tv', 'pairing_code', now(), now(), 'ps-ref')`)

	// ON DELETE SET NULL on pairing_session_id
	mustExecPool(t, pool, `DELETE FROM pairing_sessions WHERE id = 'ps-ref'`)

	var sessionID *string
	err = pool.QueryRow(context.Background(),
		`SELECT pairing_session_id FROM paired_devices WHERE id = 'dev-provisional'`).Scan(&sessionID)
	if err != nil {
		t.Fatalf("scan pairing_session_id: %v", err)
	}
	if sessionID != nil {
		t.Fatalf("expected pairing_session_id to be set to NULL after deleting pairing_session, got %v", *sessionID)
	}
}

// FR-7 / acceptance criteria: migration 00010 rolls back cleanly (down to 9)
// leaving earlier migrations intact, and re-applies cleanly.
func TestSchema_Phase13MigrationIsReversible(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	if err := postgres.Migrate(context.Background(), os.Getenv("TEST_DATABASE_URL")); err != nil {
		t.Fatalf("Migrate up: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM pairing_sessions LIMIT 1"); err != nil {
		t.Fatalf("pairing_sessions table should exist after up: %v", err)
	}

	goose.SetBaseFS(os.DirFS("migrations"))
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("SetDialect: %v", err)
	}
	if err := goose.DownToContext(context.Background(), db, ".", 9); err != nil {
		t.Fatalf("goose down to 00009: %v", err)
	}

	for _, table := range []string{"pairing_sessions", "paired_devices", "enrolment_grant_jtis", "network_settings"} {
		if tableExists(t, db, table) {
			t.Fatalf("%s table still present after down migration 00010", table)
		}
	}

	// Earlier migration tables survive
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM users LIMIT 1"); err != nil {
		t.Fatalf("earlier migration table 'users' should survive down migration: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM libraries LIMIT 1"); err != nil {
		t.Fatalf("earlier migration table 'libraries' should survive down migration: %v", err)
	}

	// Re-up
	if err := goose.UpContext(context.Background(), db, "."); err != nil {
		t.Fatalf("goose re-up 00010: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "SELECT id FROM pairing_sessions LIMIT 1"); err != nil {
		t.Fatalf("pairing_sessions table should exist again after re-up: %v", err)
	}
}
