package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// AuthorRepository is internal/persistence/postgres's
// domain.AuthorRepository implementation, the same
// two-table-with-replace-on-Save shape as WorkRepository: authors plus
// author_external_references.
type AuthorRepository struct {
	pool *pgxpool.Pool
}

func NewAuthorRepository(pool *pgxpool.Pool) *AuthorRepository {
	return &AuthorRepository{pool: pool}
}

var _ domain.AuthorRepository = (*AuthorRepository)(nil)

func (r *AuthorRepository) FindByID(ctx context.Context, id domain.AuthorID) (*domain.Author, error) {
	exec := executorFrom(ctx, r.pool)

	var name string
	var mergedInto *string
	err := exec.QueryRow(ctx,
		"SELECT name, merged_into FROM authors WHERE id = $1", string(id),
	).Scan(&name, &mergedInto)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "author not found"}
		}
		return nil, TranslateError(err)
	}

	var mergedIntoID *domain.AuthorID
	if mergedInto != nil {
		id := domain.AuthorID(*mergedInto)
		mergedIntoID = &id
	}

	refRows, err := exec.Query(ctx,
		"SELECT source, external_id FROM author_external_references WHERE author_id = $1", string(id))
	if err != nil {
		return nil, TranslateError(err)
	}
	var refs []domain.ExternalReference
	for refRows.Next() {
		var ref domain.ExternalReference
		if err := refRows.Scan(&ref.Source, &ref.ID); err != nil {
			refRows.Close()
			return nil, TranslateError(err)
		}
		refs = append(refs, ref)
	}
	if err := refRows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	return domain.RehydrateAuthor(id, name, refs, mergedIntoID), nil
}

func (r *AuthorRepository) Save(ctx context.Context, a *domain.Author) error {
	exec := executorFrom(ctx, r.pool)

	var mergedInto *string
	if a.MergedInto() != nil {
		s := string(*a.MergedInto())
		mergedInto = &s
	}

	_, err := exec.Exec(ctx, `INSERT INTO authors (id, name, merged_into)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			merged_into = EXCLUDED.merged_into`,
		string(a.ID()), a.Name(), mergedInto)
	if err != nil {
		return TranslateError(err)
	}

	if _, err := exec.Exec(ctx, "DELETE FROM author_external_references WHERE author_id = $1", string(a.ID())); err != nil {
		return TranslateError(err)
	}
	for _, ref := range a.ExternalReferences() {
		if _, err := exec.Exec(ctx,
			"INSERT INTO author_external_references (author_id, source, external_id) VALUES ($1, $2, $3)",
			string(a.ID()), ref.Source, ref.ID); err != nil {
			return TranslateError(err)
		}
	}

	return nil
}
