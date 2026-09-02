package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// ReadingProgressRepository is internal/persistence/postgres's
// domain.ReadingProgressRepository implementation (T24, R8). Save is
// upsert-by-id, the same shape as Work/Edition/Source's own Save — a
// normal update-in-place operation, unlike SourceOffering's tuple-
// identity Save. domain-reading.md FR-1's singleton-per-Work invariant
// (a *different* id for an already-recorded WorkID) is not caught by
// the id-conflict clause here; it surfaces as a real unique_violation on
// reading_progress.work_id's own UNIQUE constraint, translated to
// Conflict by TranslateError.
type ReadingProgressRepository struct {
	pool *pgxpool.Pool
}

func NewReadingProgressRepository(pool *pgxpool.Pool) *ReadingProgressRepository {
	return &ReadingProgressRepository{pool: pool}
}

var _ domain.ReadingProgressRepository = (*ReadingProgressRepository)(nil)

func (r *ReadingProgressRepository) FindByWork(ctx context.Context, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return r.findByWork(ctx, "", "", workID, "")
}

// FindByWorkForUpdate adds SELECT ... FOR UPDATE so the reconcile
// transaction's read-then-write is atomic (backend-reading-api.md FR-2).
func (r *ReadingProgressRepository) FindByWorkForUpdate(ctx context.Context, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return r.findByWork(ctx, "", "", workID, " FOR UPDATE")
}

func (r *ReadingProgressRepository) FindByWorkAndUser(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return r.findByWork(ctx, userID, libraryID, workID, "")
}

func (r *ReadingProgressRepository) FindByWorkAndUserForUpdate(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return r.findByWork(ctx, userID, libraryID, workID, " FOR UPDATE")
}

func (r *ReadingProgressRepository) findByWork(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, workID domain.WorkID, lock string) (*domain.ReadingProgress, error) {
	exec := executorFrom(ctx, r.pool)

	var id string
	var percentage float64
	var epoch int64
	var precisePositionEditionID, precisePositionValue *string
	var deviceID string
	var observedAt time.Time

	query := `SELECT id, percentage, epoch, precise_position_edition_id, precise_position_value, device_id, observed_at
		FROM reading_progress
		WHERE work_id = $1 AND COALESCE(user_id, '') = COALESCE($2, '') AND COALESCE(library_id, '') = COALESCE($3, '')` + lock

	err := exec.QueryRow(ctx, query, string(workID), string(userID), string(libraryID)).
		Scan(&id, &percentage, &epoch, &precisePositionEditionID, &precisePositionValue, &deviceID, &observedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "reading progress not found"}
		}
		return nil, TranslateError(err)
	}

	pct, err := domain.NewPercentage(percentage)
	if err != nil {
		return nil, TranslateError(err)
	}

	var precisePosition *domain.PrecisePosition
	if precisePositionEditionID != nil && precisePositionValue != nil {
		precisePosition = &domain.PrecisePosition{
			EditionID: domain.EditionID(*precisePositionEditionID),
			Value:     *precisePositionValue,
		}
	}

	return domain.RehydrateReadingProgress(
		domain.ReadingProgressID(id), workID, pct, epoch, precisePosition, domain.DeviceID(deviceID), observedAt,
	), nil
}

func (r *ReadingProgressRepository) Save(ctx context.Context, p *domain.ReadingProgress) error {
	return r.SaveForUser(ctx, "", "", p)
}

func (r *ReadingProgressRepository) SaveForUser(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, p *domain.ReadingProgress) error {
	exec := executorFrom(ctx, r.pool)

	var precisePositionEditionID, precisePositionValue *string
	if pos := p.PrecisePosition(); pos != nil {
		editionID := string(pos.EditionID)
		value := pos.Value
		precisePositionEditionID = &editionID
		precisePositionValue = &value
	}

	var uid, lid *string
	if string(userID) != "" {
		u := string(userID)
		uid = &u
	}
	if string(libraryID) != "" {
		l := string(libraryID)
		lid = &l
	}

	_, err := exec.Exec(ctx, `INSERT INTO reading_progress
			(id, work_id, user_id, library_id, percentage, epoch, precise_position_edition_id, precise_position_value, device_id, observed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			work_id = EXCLUDED.work_id,
			user_id = EXCLUDED.user_id,
			library_id = EXCLUDED.library_id,
			percentage = EXCLUDED.percentage,
			epoch = EXCLUDED.epoch,
			precise_position_edition_id = EXCLUDED.precise_position_edition_id,
			precise_position_value = EXCLUDED.precise_position_value,
			device_id = EXCLUDED.device_id,
			observed_at = EXCLUDED.observed_at`,
		string(p.ID()), string(p.WorkID()), uid, lid, float64(p.Percentage()), p.Epoch(),
		precisePositionEditionID, precisePositionValue, string(p.DeviceID()), p.ObservedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

