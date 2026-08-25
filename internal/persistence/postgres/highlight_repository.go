package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// HighlightRepository is internal/persistence/postgres's
// domain.HighlightRepository implementation (T24, R8). A single physical
// table, one row per aggregate. Save is upsert-by-id, the same shape as
// BookmarkRepository's own Save — Highlight has no uniqueness invariant
// beyond its own id.
type HighlightRepository struct {
	pool *pgxpool.Pool
}

func NewHighlightRepository(pool *pgxpool.Pool) *HighlightRepository {
	return &HighlightRepository{pool: pool}
}

var _ domain.HighlightRepository = (*HighlightRepository)(nil)

func (r *HighlightRepository) FindByID(ctx context.Context, id domain.HighlightID) (*domain.Highlight, error) {
	exec := executorFrom(ctx, r.pool)

	var editionID, startPosition, endPosition, note, category string
	err := exec.QueryRow(ctx,
		"SELECT edition_id, start_position, end_position, note, category FROM highlights WHERE id = $1", string(id),
	).Scan(&editionID, &startPosition, &endPosition, &note, &category)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "highlight not found"}
		}
		return nil, TranslateError(err)
	}

	return domain.NewHighlight(id, domain.EditionID(editionID), startPosition, endPosition, note, category), nil
}

func (r *HighlightRepository) FindByEdition(ctx context.Context, editionID domain.EditionID) ([]*domain.Highlight, error) {
	exec := executorFrom(ctx, r.pool)

	rows, err := exec.Query(ctx,
		"SELECT id, start_position, end_position, note, category FROM highlights WHERE edition_id = $1", string(editionID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.Highlight
	for rows.Next() {
		var id, startPosition, endPosition, note, category string
		if err := rows.Scan(&id, &startPosition, &endPosition, &note, &category); err != nil {
			return nil, TranslateError(err)
		}
		result = append(result, domain.NewHighlight(domain.HighlightID(id), editionID, startPosition, endPosition, note, category))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *HighlightRepository) Save(ctx context.Context, h *domain.Highlight) error {
	exec := executorFrom(ctx, r.pool)

	_, err := exec.Exec(ctx, `INSERT INTO highlights (id, edition_id, start_position, end_position, note, category)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			edition_id = EXCLUDED.edition_id,
			start_position = EXCLUDED.start_position,
			end_position = EXCLUDED.end_position,
			note = EXCLUDED.note,
			category = EXCLUDED.category`,
		string(h.ID()), string(h.EditionID()), h.StartPosition(), h.EndPosition(), h.Note(), h.Category())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *HighlightRepository) Delete(ctx context.Context, id domain.HighlightID) error {
	exec := executorFrom(ctx, r.pool)

	if _, err := exec.Exec(ctx, "DELETE FROM highlights WHERE id = $1", string(id)); err != nil {
		return TranslateError(err)
	}
	return nil
}
