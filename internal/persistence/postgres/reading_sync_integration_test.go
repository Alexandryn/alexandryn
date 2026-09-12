//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestPairedDeviceRepository_AdvanceCursor_MonotonicSQL(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('user-mono-1', 'monouser', 'mono@example.com', 'reader', now(), now())`)

	devRepo := postgres.NewPairedDeviceRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	devID := domain.DeviceID("dev-mono-1")
	device, err := domain.NewPairedDevice(
		devID,
		domain.UserID("user-mono-1"),
		"Pixel 8",
		domain.DeviceClassPhone,
		domain.EnrolledViaPairingCode,
		now,
	)
	if err != nil {
		t.Fatalf("NewPairedDevice: %v", err)
	}
	if err := devRepo.Save(ctx, device); err != nil {
		t.Fatalf("devRepo.Save: %v", err)
	}

	// 1. Initial advance to 10
	if err := devRepo.AdvanceCursor(ctx, devID, 10, now); err != nil {
		t.Fatalf("AdvanceCursor to 10: %v", err)
	}
	loaded, err := devRepo.FindByID(ctx, devID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if loaded.SyncCursor() != 10 {
		t.Fatalf("expected cursor 10, got %d", loaded.SyncCursor())
	}

	// 2. Advance to 20
	if err := devRepo.AdvanceCursor(ctx, devID, 20, now); err != nil {
		t.Fatalf("AdvanceCursor to 20: %v", err)
	}
	loaded, err = devRepo.FindByID(ctx, devID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if loaded.SyncCursor() != 20 {
		t.Fatalf("expected cursor 20, got %d", loaded.SyncCursor())
	}

	// 3. Regressive advance to 15 (should not decrease database cursor due to AND sync_cursor < $1)
	if err := devRepo.AdvanceCursor(ctx, devID, 15, now); err != nil {
		t.Fatalf("AdvanceCursor regressive: %v", err)
	}
	loaded, err = devRepo.FindByID(ctx, devID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if loaded.SyncCursor() != 20 {
		t.Fatalf("expected cursor to remain 20 after regression attempt, got %d", loaded.SyncCursor())
	}

	// 4. Stale advance to 20 (equal, should no-op)
	if err := devRepo.AdvanceCursor(ctx, devID, 20, now); err != nil {
		t.Fatalf("AdvanceCursor equal: %v", err)
	}
	loaded, err = devRepo.FindByID(ctx, devID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if loaded.SyncCursor() != 20 {
		t.Fatalf("expected cursor to remain 20 after equal attempt, got %d", loaded.SyncCursor())
	}
}

func TestReadingSyncRepository_UserAndLibraryScoping(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('u-scope-1', 'scope1', 'scope1@example.com', 'reader', now(), now()),
		       ('u-scope-2', 'scope2', 'scope2@example.com', 'reader', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO libraries (id, name, created_at, updated_at)
		VALUES ('lib-scope-1', 'Scope Lib 1', now(), now()),
		       ('lib-scope-2', 'Scope Lib 2', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO works (id, title)
		VALUES ('work-sc-1', 'Work 1'), ('work-sc-2', 'Work 2'), ('work-sc-3', 'Work 3')`)
	mustExecPool(t, pool, `INSERT INTO editions (id, work_id, language, publisher)
		VALUES ('ed-sc-1', 'work-sc-1', 'en', ''),
		       ('ed-sc-2', 'work-sc-2', 'en', ''),
		       ('ed-sc-3', 'work-sc-3', 'en', '')`)

	// Target data: User 1 in Lib 1
	mustExecPool(t, pool, `INSERT INTO reading_progress (id, work_id, user_id, library_id, percentage, epoch, device_id, observed_at)
		VALUES ('prog-target', 'work-sc-1', 'u-scope-1', 'lib-scope-1', 0.5, 0, 'dev-1', now())`)
	mustExecPool(t, pool, `INSERT INTO bookmarks (id, edition_id, user_id, library_id, position, label, created_at)
		VALUES ('bm-target', 'ed-sc-1', 'u-scope-1', 'lib-scope-1', 'epubcfi(/6/2)', 'Target Bookmark', now())`)
	mustExecPool(t, pool, `INSERT INTO highlights (id, edition_id, user_id, library_id, start_position, end_position, note, category, created_at)
		VALUES ('hl-target', 'ed-sc-1', 'u-scope-1', 'lib-scope-1', 'epubcfi(/6/2)', 'epubcfi(/6/4)', 'Target Note', '', now())`)

	// Other library: User 1 in Lib 2
	mustExecPool(t, pool, `INSERT INTO reading_progress (id, work_id, user_id, library_id, percentage, epoch, device_id, observed_at)
		VALUES ('prog-other-lib', 'work-sc-2', 'u-scope-1', 'lib-scope-2', 0.3, 0, 'dev-1', now())`)
	mustExecPool(t, pool, `INSERT INTO bookmarks (id, edition_id, user_id, library_id, position, label, created_at)
		VALUES ('bm-other-lib', 'ed-sc-2', 'u-scope-1', 'lib-scope-2', 'epubcfi(/6/2)', 'Other Lib', now())`)
	mustExecPool(t, pool, `INSERT INTO highlights (id, edition_id, user_id, library_id, start_position, end_position, note, category, created_at)
		VALUES ('hl-other-lib', 'ed-sc-2', 'u-scope-1', 'lib-scope-2', 'epubcfi(/6/2)', 'epubcfi(/6/4)', 'Other Lib Note', '', now())`)

	// Other user: User 2 in Lib 1
	mustExecPool(t, pool, `INSERT INTO reading_progress (id, work_id, user_id, library_id, percentage, epoch, device_id, observed_at)
		VALUES ('prog-other-user', 'work-sc-3', 'u-scope-2', 'lib-scope-1', 0.7, 0, 'dev-2', now())`)
	mustExecPool(t, pool, `INSERT INTO bookmarks (id, edition_id, user_id, library_id, position, label, created_at)
		VALUES ('bm-other-user', 'ed-sc-3', 'u-scope-2', 'lib-scope-1', 'epubcfi(/6/2)', 'Other User', now())`)
	mustExecPool(t, pool, `INSERT INTO highlights (id, edition_id, user_id, library_id, start_position, end_position, note, category, created_at)
		VALUES ('hl-other-user', 'ed-sc-3', 'u-scope-2', 'lib-scope-1', 'epubcfi(/6/2)', 'epubcfi(/6/4)', 'Other User Note', '', now())`)

	syncRepo := postgres.NewReadingSyncRepository(pool)
	data, err := syncRepo.GetReadingSyncData(ctx, "u-scope-1", "lib-scope-1", 0)
	if err != nil {
		t.Fatalf("GetReadingSyncData: %v", err)
	}

	if len(data.Progress) != 1 || data.Progress[0].WorkID != "work-sc-1" {
		t.Fatalf("expected 1 progress record for work-sc-1, got %+v", data.Progress)
	}
	if len(data.Bookmarks) != 1 || data.Bookmarks[0].ID != "bm-target" {
		t.Fatalf("expected 1 bookmark bm-target, got %+v", data.Bookmarks)
	}
	if len(data.Highlights) != 1 || data.Highlights[0].ID != "hl-target" {
		t.Fatalf("expected 1 highlight hl-target, got %+v", data.Highlights)
	}
}

func TestReadingSyncRepository_OutOrderInterleavedCommit_WatermarkGate(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('u-gate-1', 'gate1', 'gate1@example.com', 'reader', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO libraries (id, name, created_at, updated_at)
		VALUES ('lib-gate-1', 'Gate Lib', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO works (id, title)
		VALUES ('work-gate-a', 'Work A'), ('work-gate-b', 'Work B')`)

	syncRepo := postgres.NewReadingSyncRepository(pool)

	// Step 1: Begin Tx A and insert prog-a (takes sequence S_A). Tx A remains open.
	txA, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin txA: %v", err)
	}
	defer func() { _ = txA.Rollback(ctx) }()

	_, err = txA.Exec(ctx, `INSERT INTO reading_progress (id, work_id, user_id, library_id, percentage, epoch, device_id, observed_at)
		VALUES ('prog-a', 'work-gate-a', 'u-gate-1', 'lib-gate-1', 0.25, 0, 'dev-1', now())`)
	if err != nil {
		t.Fatalf("txA insert: %v", err)
	}

	// Step 2: Begin Tx B and insert prog-b (takes sequence S_B > S_A). Commit Tx B.
	txB, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin txB: %v", err)
	}
	_, err = txB.Exec(ctx, `INSERT INTO reading_progress (id, work_id, user_id, library_id, percentage, epoch, device_id, observed_at)
		VALUES ('prog-b', 'work-gate-b', 'u-gate-1', 'lib-gate-1', 0.75, 0, 'dev-2', now())`)
	if err != nil {
		_ = txB.Rollback(ctx)
		t.Fatalf("txB insert: %v", err)
	}
	if err := txB.Commit(ctx); err != nil {
		t.Fatalf("txB commit: %v", err)
	}

	// Step 3: Run sync while Tx A is still in-flight.
	// The watermark gate (xmin < pg_snapshot_xmin) must withhold prog-b
	// because Tx A is still open and has a lower transaction id / sequence.
	data1, err := syncRepo.GetReadingSyncData(ctx, "u-gate-1", "lib-gate-1", 0)
	if err != nil {
		t.Fatalf("GetReadingSyncData while txA in-flight: %v", err)
	}
	if len(data1.Progress) != 0 {
		t.Fatalf("expected 0 progress records while earlier transaction in-flight, got %d", len(data1.Progress))
	}
	if data1.Cursor != 0 {
		t.Fatalf("expected cursor to remain 0 while earlier transaction in-flight, got %d", data1.Cursor)
	}

	// Step 4: Commit Tx A.
	if err := txA.Commit(ctx); err != nil {
		t.Fatalf("txA commit: %v", err)
	}

	// Step 5: Run sync again.
	// Now both transactions have committed. Both prog-a and prog-b must be returned,
	// and cursor must advance to the higher sequence without dropping prog-a.
	data2, err := syncRepo.GetReadingSyncData(ctx, "u-gate-1", "lib-gate-1", 0)
	if err != nil {
		t.Fatalf("GetReadingSyncData after txA commit: %v", err)
	}
	if len(data2.Progress) != 2 {
		t.Fatalf("expected 2 progress records after both committed, got %d", len(data2.Progress))
	}
	if data2.Cursor == 0 {
		t.Fatalf("expected cursor to advance, got %d", data2.Cursor)
	}
}

// TestReadingSyncRepository_DeltaIsPageBounded is the #114 close-gate: a
// first sync with hundreds of pending rows returns at most one page, the
// cursor stops at the last delivered row, and a follow-up poll from that
// cursor drains the remainder.
func TestReadingSyncRepository_DeltaIsPageBounded(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('u-page', 'page', 'page@example.com', 'reader', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO libraries (id, name, created_at, updated_at)
		VALUES ('lib-page', 'Page Lib', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO works (id, title) VALUES ('w-page', 'W')`)
	mustExecPool(t, pool, `INSERT INTO editions (id, work_id, language, publisher) VALUES ('ed-page', 'w-page', 'en', '')`)

	const total = 620
	mustExecPool(t, pool, `INSERT INTO bookmarks (id, edition_id, user_id, library_id, position, label, created_at)
		SELECT 'bm-' || lpad(g::text, 4, '0'), 'ed-page', 'u-page', 'lib-page', 'epubcfi(/6/2)', '', now()
		FROM generate_series(1, 620) g`)

	syncRepo := postgres.NewReadingSyncRepository(pool)

	first, err := syncRepo.GetReadingSyncData(ctx, "u-page", "lib-page", 0)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Bookmarks) != 500 {
		t.Fatalf("first page returned %d bookmarks, want 500", len(first.Bookmarks))
	}
	if first.Cursor != first.Bookmarks[len(first.Bookmarks)-1].SyncSequence {
		t.Fatalf("cursor %d is not the last delivered sequence %d", first.Cursor, first.Bookmarks[len(first.Bookmarks)-1].SyncSequence)
	}

	second, err := syncRepo.GetReadingSyncData(ctx, "u-page", "lib-page", first.Cursor)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second.Bookmarks) != total-500 {
		t.Fatalf("second page returned %d bookmarks, want %d", len(second.Bookmarks), total-500)
	}

	third, err := syncRepo.GetReadingSyncData(ctx, "u-page", "lib-page", second.Cursor)
	if err != nil {
		t.Fatalf("third page: %v", err)
	}
	if len(third.Bookmarks) != 0 {
		t.Fatalf("third page returned %d bookmarks, want 0", len(third.Bookmarks))
	}
}

// TestReadingSyncRepository_ConcurrentPushPull verifies that many
// devices writing reading progress while others poll the sync delta must
// not lose a write, must each get a distinct sync_sequence, and a pull
// from cursor 0 must eventually deliver every write with no permanent
// gap.
func TestReadingSyncRepository_ConcurrentPushPull(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('u-cc', 'cc', 'cc@example.com', 'reader', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO libraries (id, name, created_at, updated_at)
		VALUES ('lib-cc', 'CC Lib', now(), now())`)
	const nWorks = 24
	mustExecPool(t, pool, `INSERT INTO works (id, title)
		SELECT 'wc-' || g, 'W' || g FROM generate_series(1, 24) g`)

	progRepo := postgres.NewReadingProgressRepository(pool)
	syncRepo := postgres.NewReadingSyncRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Readers poll the delta continuously while the writes land.
	stopPull := make(chan struct{})
	var readers sync.WaitGroup
	pullErr := make(chan error, 6)
	for i := 0; i < 6; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stopPull:
					return
				default:
				}
				if _, err := syncRepo.GetReadingSyncData(ctx, "u-cc", "lib-cc", 0); err != nil {
					pullErr <- err
					return
				}
			}
		}()
	}

	// One writer per work, all firing at once.
	var writers sync.WaitGroup
	writeErr := make(chan error, nWorks)
	for i := 1; i <= nWorks; i++ {
		writers.Add(1)
		go func(n int) {
			defer writers.Done()
			pct, _ := domain.NewPercentage(0.5)
			p := domain.RehydrateReadingProgress(
				domain.ReadingProgressID(fmt.Sprintf("rp-cc-%d", n)),
				domain.WorkID(fmt.Sprintf("wc-%d", n)), pct, 1, nil, "d", now,
			)
			if err := progRepo.SaveForUser(ctx, "u-cc", "lib-cc", p); err != nil {
				writeErr <- err
			}
		}(i)
	}

	writers.Wait()
	close(stopPull)
	readers.Wait()

	select {
	case err := <-writeErr:
		t.Fatalf("a concurrent write failed: %v", err)
	default:
	}
	select {
	case err := <-pullErr:
		t.Fatalf("a concurrent pull failed: %v", err)
	default:
	}

	var rows, distinctSeq int
	if err := pool.QueryRow(ctx,
		`SELECT count(*), count(DISTINCT sync_sequence) FROM reading_progress WHERE user_id = 'u-cc' AND library_id = 'lib-cc'`,
	).Scan(&rows, &distinctSeq); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != nWorks {
		t.Fatalf("persisted %d progress rows, want %d — a concurrent write was lost", rows, nWorks)
	}
	if distinctSeq != nWorks {
		t.Fatalf("%d distinct sync_sequence values across %d rows — a sequence was reused", distinctSeq, rows)
	}

	// Page through from the start; every write must be delivered exactly
	// once, in strictly increasing sequence order.
	seen := map[domain.WorkID]bool{}
	var lastSeq int64
	cursor := int64(0)
	for {
		data, err := syncRepo.GetReadingSyncData(ctx, "u-cc", "lib-cc", cursor)
		if err != nil {
			t.Fatalf("drain GetReadingSyncData: %v", err)
		}
		if len(data.Progress) == 0 {
			break
		}
		for _, item := range data.Progress {
			if item.SyncSequence <= lastSeq {
				t.Fatalf("sync_sequence not monotonic: %d after %d", item.SyncSequence, lastSeq)
			}
			lastSeq = item.SyncSequence
			if seen[item.WorkID] {
				t.Fatalf("work %s delivered twice", item.WorkID)
			}
			seen[item.WorkID] = true
		}
		if data.Cursor <= cursor {
			t.Fatalf("cursor did not advance: %d -> %d", cursor, data.Cursor)
		}
		cursor = data.Cursor
	}
	if len(seen) != nWorks {
		t.Fatalf("drained %d distinct works, want %d — a committed write was skipped by the watermark gate", len(seen), nWorks)
	}
}

// TestReadingProgressRepository_ConcurrentSameWorkReconcile is #137's
// contention half: many devices reporting progress for the SAME work
// under SELECT ... FOR UPDATE must serialise, so the canonical row ends
// at the highest reported value with no lost update.
func TestReadingProgressRepository_ConcurrentSameWorkReconcile(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, `INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ('u-rc', 'rc', 'rc@example.com', 'reader', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO libraries (id, name, created_at, updated_at)
		VALUES ('lib-rc', 'RC Lib', now(), now())`)
	mustExecPool(t, pool, `INSERT INTO works (id, title) VALUES ('w-rc', 'Contended')`)

	repo := postgres.NewReadingProgressRepository(pool)
	transactor := postgres.NewTransactor(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	zero, _ := domain.NewPercentage(0)
	if err := repo.SaveForUser(ctx, "u-rc", "lib-rc",
		domain.RehydrateReadingProgress("rp-rc", "w-rc", zero, 1, nil, "seed", now)); err != nil {
		t.Fatalf("seed canonical row: %v", err)
	}

	targets := []float64{0.10, 0.90, 0.30, 1.00, 0.55, 0.20, 0.75, 0.40, 0.65, 0.85}
	var wg sync.WaitGroup
	errs := make(chan error, len(targets))
	for _, tgt := range targets {
		wg.Add(1)
		go func(target float64) {
			defer wg.Done()
			err := transactor.InTx(ctx, func(txCtx context.Context) error {
				current, err := repo.FindByWorkAndUserForUpdate(txCtx, "u-rc", "lib-rc", "w-rc")
				if err != nil {
					return err
				}
				if target <= float64(current.Percentage()) {
					return nil // a higher value already won
				}
				next, _ := domain.NewPercentage(target)
				return repo.SaveForUser(txCtx, "u-rc", "lib-rc",
					domain.RehydrateReadingProgress(current.ID(), "w-rc", next, current.Epoch(), nil, "d", now))
			})
			if err != nil {
				errs <- err
			}
		}(tgt)
	}
	wg.Wait()

	select {
	case err := <-errs:
		t.Fatalf("a reconcile transaction failed: %v", err)
	default:
	}

	final, err := repo.FindByWorkAndUser(ctx, "u-rc", "lib-rc", "w-rc")
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if float64(final.Percentage()) != 1.00 {
		t.Fatalf("canonical percentage = %v, want 1.00 — a higher report was lost under contention", float64(final.Percentage()))
	}
}
