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

type ReadingSyncRepository struct {
	pool *pgxpool.Pool
}

func NewReadingSyncRepository(pool *pgxpool.Pool) *ReadingSyncRepository {
	return &ReadingSyncRepository{pool: pool}
}

func (r *ReadingSyncRepository) GetReadingSyncData(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, since int64) (*ReadingSyncData, error) {
	exec := executorFrom(ctx, r.pool)

	res := &ReadingSyncData{
		Cursor:     since,
		Progress:   make([]SyncProgressItem, 0),
		Bookmarks:  make([]SyncBookmarkItem, 0),
		Highlights: make([]SyncHighlightItem, 0),
	}
	maxSeq := since

	// 1. Reading progress
	progressRows, err := exec.Query(ctx, `
		SELECT work_id, percentage, epoch, precise_position_edition_id, precise_position_value, device_id, observed_at, sync_sequence
		FROM reading_progress
		WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3
		ORDER BY sync_sequence ASC`,
		string(userID), string(libraryID), since,
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

	// 2. Bookmarks
	bookmarkRows, err := exec.Query(ctx, `
		SELECT id, edition_id, position, label, created_at, sync_sequence
		FROM bookmarks
		WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3
		ORDER BY sync_sequence ASC`,
		string(userID), string(libraryID), since,
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

	// 3. Highlights
	highlightRows, err := exec.Query(ctx, `
		SELECT id, edition_id, start_position, end_position, note, category, created_at, sync_sequence
		FROM highlights
		WHERE user_id = $1 AND library_id = $2 AND sync_sequence > $3
		ORDER BY sync_sequence ASC`,
		string(userID), string(libraryID), since,
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

	res.Cursor = maxSeq
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
