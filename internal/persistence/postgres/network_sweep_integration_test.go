//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestNetworkSweep_SweepOnce_TransitionsAndDeletions(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-sweep-admin', 'sweepadmin', 'sweepadmin@example.com', 'admin', now(), now()),
		       ('user-sweep-reader', 'sweepreader', 'sweepreader@example.com', 'reader', now(), now())`)

	now := time.Now().UTC().Truncate(time.Microsecond)

	// a) Stale pending session: expires_at in past -> should become expired
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-stale-pending', 'user-sweep-admin', '\x01', '\x10', 'pending', now() - interval '6 minutes', now() - interval '1 minute')`)

	// b) Active pending session: expires_at in future -> should remain pending
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-active-pending', 'user-sweep-admin', '\x02', '\x11', 'pending', now(), now() + interval '4 minutes')`)

	// c) Expired session > 24h old -> should be deleted
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-old-expired', 'user-sweep-admin', '\x03', '\x12', 'expired', now() - interval '26 hours', now() - interval '25 hours')`)

	// d) Consumed session > 24h old -> should be deleted
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-old-consumed', 'user-sweep-admin', '\x04', '\x13', 'consumed', now() - interval '26 hours', now() - interval '25 hours')`)

	// e) Consumed session < 24h old -> should survive
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-recent-consumed', 'user-sweep-admin', '\x05', '\x14', 'consumed', now() - interval '3 hours', now() - interval '2 hours')`)

	// f) Provisional device > 1h old -> should be deleted
	mustExecPool(t, pool, `INSERT INTO paired_devices
		(id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at)
		VALUES ('dev-old-provisional', NULL, 'Old Prov', 'phone', 'pairing_code', now() - interval '70 minutes', now() - interval '70 minutes')`)

	// g) Provisional device < 1h old -> should survive
	mustExecPool(t, pool, `INSERT INTO paired_devices
		(id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at)
		VALUES ('dev-recent-provisional', NULL, 'Recent Prov', 'phone', 'pairing_code', now() - interval '10 minutes', now() - interval '10 minutes')`)

	// h) Claimed device > 1h old -> should survive
	mustExecPool(t, pool, `INSERT INTO paired_devices
		(id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at)
		VALUES ('dev-old-claimed', 'user-sweep-reader', 'Old Claimed', 'phone', 'pairing_code', now() - interval '70 minutes', now() - interval '70 minutes')`)

	// i) JTI > 10m old (+1h margin) -> should be deleted
	mustExecPool(t, pool, `INSERT INTO enrolment_grant_jtis (jti, spent_at)
		VALUES ('jti-old-spent', now() - interval '75 minutes')`)

	// j) JTI < 10m old -> should survive
	mustExecPool(t, pool, `INSERT INTO enrolment_grant_jtis (jti, spent_at)
		VALUES ('jti-recent-spent', now() - interval '5 minutes')`)

	sweep := postgres.NewNetworkSweep(pool)
	res, err := sweep.SweepOnce(ctx, now)
	if err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	if res.ExpiredSessions != 1 {
		t.Errorf("res.ExpiredSessions = %d, want 1", res.ExpiredSessions)
	}
	if res.DeletedSessions != 2 {
		t.Errorf("res.DeletedSessions = %d, want 2", res.DeletedSessions)
	}
	if res.DeletedDevices != 1 {
		t.Errorf("res.DeletedDevices = %d, want 1", res.DeletedDevices)
	}
	if res.DeletedGrantJTIs != 1 {
		t.Errorf("res.DeletedGrantJTIs = %d, want 1", res.DeletedGrantJTIs)
	}

	// Direct DB verifications:
	db := testDB(t)

	// a) ps-stale-pending is now 'expired'
	var staleState string
	err = db.QueryRowContext(ctx, `SELECT state FROM pairing_sessions WHERE id = 'ps-stale-pending'`).Scan(&staleState)
	if err != nil || staleState != "expired" {
		t.Errorf("ps-stale-pending state = %q (err: %v), want expired", staleState, err)
	}

	// b) ps-active-pending is still 'pending'
	var activeState string
	err = db.QueryRowContext(ctx, `SELECT state FROM pairing_sessions WHERE id = 'ps-active-pending'`).Scan(&activeState)
	if err != nil || activeState != "pending" {
		t.Errorf("ps-active-pending state = %q (err: %v), want pending", activeState, err)
	}

	// c, d) old sessions deleted
	var count int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM pairing_sessions WHERE id IN ('ps-old-expired', 'ps-old-consumed')`).Scan(&count)
	if count != 0 {
		t.Errorf("old sessions count = %d, want 0", count)
	}

	// e) recent consumed session survives
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM pairing_sessions WHERE id = 'ps-recent-consumed'`).Scan(&count)
	if count != 1 {
		t.Errorf("recent consumed session count = %d, want 1", count)
	}

	// f) old provisional device deleted
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM paired_devices WHERE id = 'dev-old-provisional'`).Scan(&count)
	if count != 0 {
		t.Errorf("dev-old-provisional count = %d, want 0", count)
	}

	// g, h) recent provisional and old claimed devices survive
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM paired_devices WHERE id IN ('dev-recent-provisional', 'dev-old-claimed')`).Scan(&count)
	if count != 2 {
		t.Errorf("surviving devices count = %d, want 2", count)
	}

	// i) old jti deleted
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM enrolment_grant_jtis WHERE jti = 'jti-old-spent'`).Scan(&count)
	if count != 0 {
		t.Errorf("old jti count = %d, want 0", count)
	}

	// j) recent jti survives
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM enrolment_grant_jtis WHERE jti = 'jti-recent-spent'`).Scan(&count)
	if count != 1 {
		t.Errorf("recent jti count = %d, want 1", count)
	}
}

func TestNetworkSweep_Start(t *testing.T) {
	pool := schemaTestPool(t)

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-start-admin', 'startadmin', 'startadmin@example.com', 'admin', now(), now())`)

	// Stale pending session: expires_at in past -> should become expired by background ticker
	mustExecPool(t, pool, `INSERT INTO pairing_sessions
		(id, initiated_by, code_ciphertext, code_index, state, created_at, expires_at)
		VALUES ('ps-start-pending', 'user-start-admin', '\x01', '\x20', 'pending', now() - interval '6 minutes', now() - interval '1 minute')`)

	sweep := postgres.NewNetworkSweep(pool)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		sweep.Start(ctx, 20*time.Millisecond, nil)
		close(done)
	}()

	db := testDB(t)
	// Poll until swept or timeout
	deadline := time.Now().Add(2 * time.Second)
	swept := false
	for time.Now().Before(deadline) {
		var state string
		err := db.QueryRowContext(ctx, `SELECT state FROM pairing_sessions WHERE id = 'ps-start-pending'`).Scan(&state)
		if err == nil && state == "expired" {
			swept = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if !swept {
		t.Fatal("expected stale session to be swept to expired by Start loop")
	}

	cancel()
	select {
	case <-done:
		// Clean exit
	case <-time.After(time.Second):
		t.Fatal("sweep.Start did not terminate after context cancel")
	}
}
