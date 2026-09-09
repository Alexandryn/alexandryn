package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

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
	var createdAt time.Time
	err := exec.QueryRow(ctx,
		"SELECT edition_id, start_position, end_position, note, category, created_at FROM highlights WHERE id = $1", string(id),
	).Scan(&editionID, &startPosition, &endPosition, &note, &category, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "highlight not found"}
		}
		return nil, TranslateError(err)
	}

	return domain.NewHighlight(id, domain.EditionID(editionID), startPosition, endPosition, note, category, createdAt), nil
}

// FindByIDAndUser returns the highlight only when it belongs to userID.
// A missing and a foreign row are both NotFound (AUDIT-0012-C1).
func (r *HighlightRepository) FindByIDAndUser(ctx context.Context, userID domain.UserID, id domain.HighlightID) (*domain.Highlight, error) {
	exec := executorFrom(ctx, r.pool)

	clauses := []string{"id = $1"}
	args := []any{string(id)}
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))

	var editionID, startPosition, endPosition, note, category string
	var createdAt time.Time
	err := exec.QueryRow(ctx,
		`SELECT edition_id, start_position, end_position, note, category, created_at FROM highlights WHERE `+strings.Join(clauses, " AND "),
		args...,
	).Scan(&editionID, &startPosition, &endPosition, &note, &category, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "highlight not found"}
		}
		return nil, TranslateError(err)
	}
	return domain.NewHighlight(id, domain.EditionID(editionID), startPosition, endPosition, note, category, createdAt), nil
}

func (r *HighlightRepository) FindByEdition(ctx context.Context, editionID domain.EditionID) ([]*domain.Highlight, error) {
	return r.FindByEditionAndUser(ctx, "", "", editionID)
}

func (r *HighlightRepository) FindByEditionAndUser(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, editionID domain.EditionID) ([]*domain.Highlight, error) {
	exec := executorFrom(ctx, r.pool)

	clauses := []string{"edition_id = $1"}
	args := []any{string(editionID)}
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))
	clauses, args = appendOwnerScope(clauses, args, "library_id", string(libraryID))

	query := `SELECT id, start_position, end_position, note, category, created_at
		FROM highlights
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY created_at, id`
	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.Highlight
	for rows.Next() {
		var id, startPosition, endPosition, note, category string
		var createdAt time.Time
		if err := rows.Scan(&id, &startPosition, &endPosition, &note, &category, &createdAt); err != nil {
			return nil, TranslateError(err)
		}
		result = append(result, domain.NewHighlight(domain.HighlightID(id), editionID, startPosition, endPosition, note, category, createdAt))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *HighlightRepository) Save(ctx context.Context, h *domain.Highlight) error {
	return r.SaveForUser(ctx, "", "", h)
}

func (r *HighlightRepository) SaveForUser(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, h *domain.Highlight) error {
	exec := executorFrom(ctx, r.pool)

	var uid, lid *string
	if string(userID) != "" {
		u := string(userID)
		uid = &u
	}
	if string(libraryID) != "" {
		l := string(libraryID)
		lid = &l
	}

	_, err := exec.Exec(ctx, `INSERT INTO highlights (id, edition_id, user_id, library_id, start_position, end_position, note, category, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			edition_id = EXCLUDED.edition_id,
			user_id = EXCLUDED.user_id,
			library_id = EXCLUDED.library_id,
			start_position = EXCLUDED.start_position,
			end_position = EXCLUDED.end_position,
			note = EXCLUDED.note,
			category = EXCLUDED.category`,
		string(h.ID()), string(h.EditionID()), uid, lid, h.StartPosition(), h.EndPosition(), h.Note(), h.Category(), h.CreatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// UpdateNoteCategoryAndUser updates note/category on a row the user owns,
// leaving edition_id and library_id untouched so a PATCH cannot relocate
// the highlight (PR #78 review). A foreign or missing id → NotFound.
func (r *HighlightRepository) UpdateNoteCategoryAndUser(ctx context.Context, userID domain.UserID, id domain.HighlightID, note, category string) error {
	exec := executorFrom(ctx, r.pool)

	clauses := []string{"id = $1"}
	args := []any{string(id), note, category}
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))

	tag, err := exec.Exec(ctx,
		`UPDATE highlights SET note = $2, category = $3 WHERE `+strings.Join(clauses, " AND "),
		args...)
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "highlight not found"}
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

// DeleteAndUser deletes only a row owned by userID (AUDIT-0012-C1).
func (r *HighlightRepository) DeleteAndUser(ctx context.Context, userID domain.UserID, id domain.HighlightID) error {
	exec := executorFrom(ctx, r.pool)

	clauses := []string{"id = $1"}
	args := []any{string(id)}
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))

	tag, err := exec.Exec(ctx,
		`DELETE FROM highlights WHERE `+strings.Join(clauses, " AND "),
		args...)
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "highlight not found"}
	}
	return nil
}
