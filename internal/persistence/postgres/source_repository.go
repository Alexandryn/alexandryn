package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// SourceRepository is internal/persistence/postgres's
// domain.SourceRepository implementation (T24, R7). A single physical
// table, one row per aggregate — no child tables. Save is upsert-by-id,
// the same shape as WorkRepository/EditionRepository's own Save, per
// T24's open question (no real caller exists yet to validate against).
type SourceRepository struct {
	pool *pgxpool.Pool
}

func NewSourceRepository(pool *pgxpool.Pool) *SourceRepository {
	return &SourceRepository{pool: pool}
}

var _ domain.SourceRepository = (*SourceRepository)(nil)

func (r *SourceRepository) FindByID(ctx context.Context, id domain.SourceID) (*domain.Source, error) {
	exec := executorFrom(ctx, r.pool)

	var label, kind string
	var canList, canSearch, canDownload bool
	err := exec.QueryRow(ctx,
		"SELECT label, can_list, can_search, can_download, kind FROM sources WHERE id = $1", string(id),
	).Scan(&label, &canList, &canSearch, &canDownload, &kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "source not found"}
		}
		return nil, TranslateError(err)
	}

	caps := domain.SourceCapabilities{CanList: canList, CanSearch: canSearch, CanDownload: canDownload}
	s, err := domain.NewSource(id, label, caps, kind)
	if err != nil {
		return nil, TranslateError(err)
	}
	return s, nil
}

func (r *SourceRepository) Save(ctx context.Context, s *domain.Source) error {
	exec := executorFrom(ctx, r.pool)

	caps := s.Capabilities()
	_, err := exec.Exec(ctx, `INSERT INTO sources (id, label, can_list, can_search, can_download, kind)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			label = EXCLUDED.label,
			can_list = EXCLUDED.can_list,
			can_search = EXCLUDED.can_search,
			can_download = EXCLUDED.can_download,
			kind = EXCLUDED.kind`,
		string(s.ID()), s.Label(), caps.CanList, caps.CanSearch, caps.CanDownload, s.Kind())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *SourceRepository) Delete(ctx context.Context, id domain.SourceID) error {
	exec := executorFrom(ctx, r.pool)

	if _, err := exec.Exec(ctx, "DELETE FROM sources WHERE id = $1", string(id)); err != nil {
		return TranslateError(err)
	}
	return nil
}
