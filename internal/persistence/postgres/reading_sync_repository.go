package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SyncProgressItem struct {
	WorkID          domain.WorkID           `json:"workId"`
	Percentage      float64                 `json:"percentage"`
	Epoch           int64                   `json:"epoch"`
	PrecisePosition *domain.PrecisePosition `json:"precisePosition,omitempty"`
	DeviceID        domain.DeviceID         `json:"deviceId"`
	ObservedAt      time.Time               `json:"observedAt"`
	SyncSequence    int64                   `json:"syncSequence"`
}

type SyncBookmarkItem struct {
	ID           string    `json:"id"`
	EditionID    string    `json:"editionId"`
	Position     string    `json:"position"`
	Label        string    `json:"label"`
	CreatedAt    time.Time `json:"createdAt"`
	SyncSequence int64     `json:"syncSequence"`
}

type SyncHighlightItem struct {
	ID            string    `json:"id"`
	EditionID     string    `json:"editionId"`
	StartPosition string    `json:"startPosition"`
	EndPosition   string    `json:"endPosition"`
	Note          string    `json:"note"`
	Category      string    `json:"category"`
	CreatedAt     time.Time `json:"createdAt"`
	SyncSequence  int64     `json:"syncSequence"`
}

type ReadingSyncData struct {
	Cursor     int64               `json:"cursor"`
	Progress   []SyncProgressItem  `json:"progress"`
	Bookmarks  []SyncBookmarkItem  `json:"bookmarks"`
	Highlights []SyncHighlightItem `json:"highlights"`
}

// syncDeltaLimit bounds each of the three delta queries so a first sync
// (since = 0) or a long-idle device cannot pull the entire history in one
// RepeatableRead transaction (#114). The device keeps polling until a
// response carries no rows; the cursor never advances past a sequence
// below which every table has been fully delivered.
const syncDeltaLimit = 500

type ReadingSyncRepository struct {
	pool *pgxpool.Pool
}

func NewReadingSyncRepository(pool *pgxpool.Pool) *ReadingSyncRepository {
	return &ReadingSyncRepository{pool: pool}
}

func (r *ReadingSyncRepository) GetReadingSyncData(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, since int64) (res *ReadingSyncData, err error) {
	var exec querier = r.pool
	var tx pgx.Tx
	if _, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		exec = executorFrom(ctx, r.pool)
	} else if r.pool != nil {
		var txErr error
		tx, txErr = r.pool.BeginTx(ctx, pgx.TxOptions{
			IsoLevel:   pgx.RepeatableRead,
			AccessMode: pgx.ReadOnly,
		})
		if txErr != nil {
			return nil, TranslateError(txErr)
		}
		defer func() {
			if p := recover(); p != nil {
				_ = tx.Rollback(ctx)
				panic(p)
			}
			if err != nil {
				_ = tx.Rollback(ctx)
			}
		}()
		exec = tx
	}

	res = &ReadingSyncData{
		Cursor:     since,
		Progress:   make([]SyncProgressItem, 0),
		Bookmarks:  make([]SyncBookmarkItem, 0),
		Highlights: make([]SyncHighlightItem, 0),
	}
	maxSeq := since

	// CDC Watermark Gating and xmin/xid8 wraparound:
	// To prevent CDC watermark skipping when concurrent transactions commit out-of-order
	// (e.g. Transaction A acquires sync_seq 10 and remains in-flight while Transaction B
	// acquires sync_seq 11 and commits, causing a sync reading at sequence 11 to miss sequence 10),
	// we constrain delta retrieval by the snapshot low-water mark:
	//
	//   AND (xmin::text::bigint < (pg_snapshot_xmin(pg_current_snapshot())::text)::bigint)
	//
	// pg_snapshot_xmin returns the lowest transaction ID (xid8) that was still active (in-flight)
	// when the current snapshot was taken. Rows inserted or updated by transactions with xmin >= snapshot_xmin
	// were either concurrent with or newer than the earliest active transaction, so they are held back until
	// all earlier transactions have completed.
	//
	// LIMITATION & LIFETIME ASSUMPTION:
	// PostgreSQL's tuple system column `xmin` is a 32-bit `xid`, whereas `pg_snapshot_xmin` returns a 64-bit `xid8`
	// incorporating the 32-bit wraparound epoch counter. The cast `::text::bigint` compares raw numeric values
	// and is sound during the cluster's first xid epoch (up to 2^31 ~ 2.1 billion write transactions). In an Alexandryn
	// single-instance or small-cluster deployment, reaching 2^31 transactions represents decades of typical usage.
	// Should an instance undergo an xid wraparound epoch, PostgreSQL VACUUM advances frozen xids, but `xmin` resets
	// to 0 while `xid8` continues to climb. Future phases requiring multi-billion transaction scale should migrate
	// sync_sequence assignment to an explicit commit-timestamp or 64-bit txid mapping table.

	// 1. Reading progress with xmin low-water gate ensuring no in-flight earlier sequence is skipped
	progressRows, err := exec.Query(ctx, `
		SELECT work_id, percentage, epoch, precise_position_edition_id, precise_position_value, device_id, observed_at, sync_sequence
		FROM reading_progress
		WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3
		  AND (xmin::text::bigint < (pg_snapshot_xmin(pg_current_snapshot())::text)::bigint)
		ORDER BY sync_sequence ASC
		LIMIT $4`,
		string(userID), string(libraryID), since, syncDeltaLimit,
	)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer progressRows.Close()

	for progressRows.Next() {
		var workID, deviceID string
		var percentage float64
		var epoch, syncSeq int64
		var posEditionID, posValue *string
		var observedAt time.Time

		if err := progressRows.Scan(&workID, &percentage, &epoch, &posEditionID, &posValue, &deviceID, &observedAt, &syncSeq); err != nil {
			return nil, TranslateError(err)
		}

		var pos *domain.PrecisePosition
		if posEditionID != nil && posValue != nil {
			pos = &domain.PrecisePosition{
				EditionID: domain.EditionID(*posEditionID),
				Value:     *posValue,
			}
		}

		res.Progress = append(res.Progress, SyncProgressItem{
			WorkID:          domain.WorkID(workID),
			Percentage:      percentage,
			Epoch:           epoch,
			PrecisePosition: pos,
			DeviceID:        domain.DeviceID(deviceID),
			ObservedAt:      observedAt,
			SyncSequence:    syncSeq,
		})
		if syncSeq > maxSeq {
			maxSeq = syncSeq
		}
	}
	if err := progressRows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	// 2. Bookmarks with xmin low-water gate
	bookmarkRows, err := exec.Query(ctx, `
		SELECT id, edition_id, position, label, created_at, sync_sequence
		FROM bookmarks
		WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3
		  AND (xmin::text::bigint < (pg_snapshot_xmin(pg_current_snapshot())::text)::bigint)
		ORDER BY sync_sequence ASC
		LIMIT $4`,
		string(userID), string(libraryID), since, syncDeltaLimit,
	)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer bookmarkRows.Close()

	for bookmarkRows.Next() {
		var id, editionID, position, label string
		var createdAt time.Time
		var syncSeq int64

		if err := bookmarkRows.Scan(&id, &editionID, &position, &label, &createdAt, &syncSeq); err != nil {
			return nil, TranslateError(err)
		}

		res.Bookmarks = append(res.Bookmarks, SyncBookmarkItem{
			ID:           id,
			EditionID:    editionID,
			Position:     position,
			Label:        label,
			CreatedAt:    createdAt,
			SyncSequence: syncSeq,
		})
		if syncSeq > maxSeq {
			maxSeq = syncSeq
		}
	}
	if err := bookmarkRows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	// 3. Highlights with xmin low-water gate
	highlightRows, err := exec.Query(ctx, `
		SELECT id, edition_id, start_position, end_position, note, category, created_at, sync_sequence
		FROM highlights
		WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3
		  AND (xmin::text::bigint < (pg_snapshot_xmin(pg_current_snapshot())::text)::bigint)
		ORDER BY sync_sequence ASC
		LIMIT $4`,
		string(userID), string(libraryID), since, syncDeltaLimit,
	)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer highlightRows.Close()

	for highlightRows.Next() {
		var id, editionID, startPos, endPos, note, category string
		var createdAt time.Time
		var syncSeq int64

		if err := highlightRows.Scan(&id, &editionID, &startPos, &endPos, &note, &category, &createdAt, &syncSeq); err != nil {
			return nil, TranslateError(err)
		}

		res.Highlights = append(res.Highlights, SyncHighlightItem{
			ID:            id,
			EditionID:     editionID,
			StartPosition: startPos,
			EndPosition:   endPos,
			Note:          note,
			Category:      category,
			CreatedAt:     createdAt,
			SyncSequence:  syncSeq,
		})
		if syncSeq > maxSeq {
			maxSeq = syncSeq
		}
	}
	if err := highlightRows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	if tx != nil {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, TranslateError(commitErr)
		}
	}

	// When a table hit the row limit, the delta is only complete up to
	// that table's last delivered sequence — advancing the cursor past it
	// would skip the rows the LIMIT cut off. Cap the cursor at the lowest
	// such boundary; the device re-polls and receives the remainder
	// (rows already delivered are idempotent upserts on the client).
	cursor := maxSeq
	if n := len(res.Progress); n == syncDeltaLimit {
		cursor = min(cursor, res.Progress[n-1].SyncSequence)
	}
	if n := len(res.Bookmarks); n == syncDeltaLimit {
		cursor = min(cursor, res.Bookmarks[n-1].SyncSequence)
	}
	if n := len(res.Highlights); n == syncDeltaLimit {
		cursor = min(cursor, res.Highlights[n-1].SyncSequence)
	}
	res.Cursor = cursor
	return res, nil
}

func (r *ReadingSyncRepository) GetProgressSyncSequence(ctx context.Context, progressID domain.ReadingProgressID) (int64, error) {
	exec := executorFrom(ctx, r.pool)
	var syncSeq int64
	err := exec.QueryRow(ctx, `SELECT sync_sequence FROM reading_progress WHERE id = $1`, string(progressID)).Scan(&syncSeq)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, &domain.Error{Category: domain.NotFound, Message: "reading progress not found"}
		}
		return 0, TranslateError(err)
	}
	return syncSeq, nil
}

// GetSyncSequenceCeiling returns the highest committed sequence across all sync tables.
// Reads the max committed sync_sequence rather than the non-transactional sequence
// allocation counter, which could be ahead of the highest committed sync_sequence.
func (r *ReadingSyncRepository) GetSyncSequenceCeiling(ctx context.Context) (int64, error) {
	exec := executorFrom(ctx, r.pool)
	var maxVal int64
	query := `
		SELECT GREATEST(
			COALESCE((SELECT MAX(sync_sequence) FROM reading_progress), 0),
			COALESCE((SELECT MAX(sync_sequence) FROM reading_bookmarks), 0),
			COALESCE((SELECT MAX(sync_sequence) FROM reading_highlights), 0)
		)`
	err := exec.QueryRow(ctx, query).Scan(&maxVal)
	if err != nil {
		return 0, TranslateError(err)
	}
	return maxVal, nil
}
