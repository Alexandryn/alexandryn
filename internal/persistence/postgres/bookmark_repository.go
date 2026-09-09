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
	var createdAt time.Time
	err := exec.QueryRow(ctx,
		"SELECT edition_id, position, label, created_at FROM bookmarks WHERE id = $1", string(id),
	).Scan(&editionID, &position, &label, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "bookmark not found"}
		}
		return nil, TranslateError(err)
	}

	return domain.NewBookmark(id, domain.EditionID(editionID), position, label, createdAt), nil
}

// FindByIDAndUser returns the bookmark only when it belongs to userID.
// A missing row and a foreign row are both NotFound — no cross-user
// existence oracle (AUDIT-0012-C1).
func (r *BookmarkRepository) FindByIDAndUser(ctx context.Context, userID domain.UserID, id domain.BookmarkID) (*domain.Bookmark, error) {
	exec := executorFrom(ctx, r.pool)

	clauses := []string{"id = $1"}
	args := []any{string(id)}
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))

	var editionID, position, label string
	var createdAt time.Time
	err := exec.QueryRow(ctx,
		`SELECT edition_id, position, label, created_at FROM bookmarks WHERE `+strings.Join(clauses, " AND "),
		args...,
	).Scan(&editionID, &position, &label, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "bookmark not found"}
		}
		return nil, TranslateError(err)
	}
	return domain.NewBookmark(id, domain.EditionID(editionID), position, label, createdAt), nil
}

func (r *BookmarkRepository) FindByEdition(ctx context.Context, editionID domain.EditionID) ([]*domain.Bookmark, error) {
	return r.FindByEditionAndUser(ctx, "", "", editionID)
}

func (r *BookmarkRepository) FindByEditionAndUser(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, editionID domain.EditionID) ([]*domain.Bookmark, error) {
	exec := executorFrom(ctx, r.pool)

	clauses := []string{"edition_id = $1"}
	args := []any{string(editionID)}
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))
	clauses, args = appendOwnerScope(clauses, args, "library_id", string(libraryID))

	query := `SELECT id, position, label, created_at
		FROM bookmarks
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY created_at, id`
	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.Bookmark
	for rows.Next() {
		var id, position, label string
		var createdAt time.Time
		if err := rows.Scan(&id, &position, &label, &createdAt); err != nil {
			return nil, TranslateError(err)
		}
		result = append(result, domain.NewBookmark(domain.BookmarkID(id), editionID, position, label, createdAt))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *BookmarkRepository) Save(ctx context.Context, b *domain.Bookmark) error {
	return r.SaveForUser(ctx, "", "", b)
}

func (r *BookmarkRepository) SaveForUser(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, b *domain.Bookmark) error {
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

	_, err := exec.Exec(ctx, `INSERT INTO bookmarks (id, edition_id, user_id, library_id, position, label, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			edition_id = EXCLUDED.edition_id,
			user_id = EXCLUDED.user_id,
			library_id = EXCLUDED.library_id,
			position = EXCLUDED.position,
			label = EXCLUDED.label`,
		string(b.ID()), string(b.EditionID()), uid, lid, b.Position(), b.Label(), b.CreatedAt())
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

// DeleteAndUser deletes only a row owned by userID. A foreign or missing
// id affects no rows and returns NotFound (AUDIT-0012-C1).
func (r *BookmarkRepository) DeleteAndUser(ctx context.Context, userID domain.UserID, id domain.BookmarkID) error {
	exec := executorFrom(ctx, r.pool)

	clauses := []string{"id = $1"}
	args := []any{string(id)}
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))

	tag, err := exec.Exec(ctx,
		`DELETE FROM bookmarks WHERE `+strings.Join(clauses, " AND "),
		args...)
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "bookmark not found"}
	}
	return nil
}
