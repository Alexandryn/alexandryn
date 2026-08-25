package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// BookmarkRepository is internal/persistence/postgres's
// domain.BookmarkRepository implementation (T24, R8). A single physical
// table, one row per aggregate. Save is upsert-by-id, the same shape as
// Work/Edition/Source's own Save — Bookmark has no uniqueness invariant
// beyond its own id.
type BookmarkRepository struct {
	pool *pgxpool.Pool
}

func NewBookmarkRepository(pool *pgxpool.Pool) *BookmarkRepository {
	return &BookmarkRepository{pool: pool}
}

var _ domain.BookmarkRepository = (*BookmarkRepository)(nil)

func (r *BookmarkRepository) FindByID(ctx context.Context, id domain.BookmarkID) (*domain.Bookmark, error) {
	exec := executorFrom(ctx, r.pool)

	var editionID, position, label string
	err := exec.QueryRow(ctx,
		"SELECT edition_id, position, label FROM bookmarks WHERE id = $1", string(id),
	).Scan(&editionID, &position, &label)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "bookmark not found"}
		}
		return nil, TranslateError(err)
	}

	return domain.NewBookmark(id, domain.EditionID(editionID), position, label), nil
}

func (r *BookmarkRepository) FindByEdition(ctx context.Context, editionID domain.EditionID) ([]*domain.Bookmark, error) {
	exec := executorFrom(ctx, r.pool)

	rows, err := exec.Query(ctx,
		"SELECT id, position, label FROM bookmarks WHERE edition_id = $1", string(editionID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.Bookmark
	for rows.Next() {
		var id, position, label string
		if err := rows.Scan(&id, &position, &label); err != nil {
			return nil, TranslateError(err)
		}
		result = append(result, domain.NewBookmark(domain.BookmarkID(id), editionID, position, label))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *BookmarkRepository) Save(ctx context.Context, b *domain.Bookmark) error {
	exec := executorFrom(ctx, r.pool)

	_, err := exec.Exec(ctx, `INSERT INTO bookmarks (id, edition_id, position, label)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			edition_id = EXCLUDED.edition_id,
			position = EXCLUDED.position,
			label = EXCLUDED.label`,
		string(b.ID()), string(b.EditionID()), b.Position(), b.Label())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *BookmarkRepository) Delete(ctx context.Context, id domain.BookmarkID) error {
	exec := executorFrom(ctx, r.pool)

	if _, err := exec.Exec(ctx, "DELETE FROM bookmarks WHERE id = $1", string(id)); err != nil {
		return TranslateError(err)
	}
	return nil
}
