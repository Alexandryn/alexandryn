package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// EditionRepository is internal/persistence/postgres's
// domain.EditionRepository implementation (T24, R5). An Edition spans
// two physical tables (editions, edition_external_references); Save
// replaces every external-reference row for this Edition's id on each
// call, the same replace-on-Save shape as WorkRepository/
// AuthorRepository (R4). Unlike Work/Author, Edition has no
// storage-only field a validating constructor can't accept — every field
// NewEdition takes is exactly what a row holds, so reads reuse NewEdition
// directly rather than needing a Rehydrate* path.
type EditionRepository struct {
	pool *pgxpool.Pool
}

func NewEditionRepository(pool *pgxpool.Pool) *EditionRepository {
	return &EditionRepository{pool: pool}
}

var _ domain.EditionRepository = (*EditionRepository)(nil)

func (r *EditionRepository) FindByID(ctx context.Context, id domain.EditionID) (*domain.Edition, error) {
	return r.load(ctx, executorFrom(ctx, r.pool), id)
}

func (r *EditionRepository) FindByWork(ctx context.Context, workID domain.WorkID) ([]*domain.Edition, error) {
	exec := executorFrom(ctx, r.pool)

	rows, err := exec.Query(ctx, "SELECT id FROM editions WHERE work_id = $1", string(workID))
	if err != nil {
		return nil, TranslateError(err)
	}
	var ids []domain.EditionID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, TranslateError(err)
		}
		ids = append(ids, domain.EditionID(id))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	result := make([]*domain.Edition, 0, len(ids))
	for _, id := range ids {
		e, err := r.load(ctx, exec, id)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, nil
}

func (r *EditionRepository) Save(ctx context.Context, e *domain.Edition) error {
	exec := executorFrom(ctx, r.pool)

	_, err := exec.Exec(ctx, `INSERT INTO editions (id, work_id, language, isbn, publisher, publication_year)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			work_id = EXCLUDED.work_id,
			language = EXCLUDED.language,
			isbn = EXCLUDED.isbn,
			publisher = EXCLUDED.publisher,
			publication_year = EXCLUDED.publication_year`,
		string(e.ID()), string(e.WorkID()), e.Language().String(), e.ISBN(), e.Publisher(), e.PublicationYear())
	if err != nil {
		return TranslateError(err)
	}

	if _, err := exec.Exec(ctx, "DELETE FROM edition_external_references WHERE edition_id = $1", string(e.ID())); err != nil {
		return TranslateError(err)
	}
	for _, ref := range e.ExternalReferences() {
		if _, err := exec.Exec(ctx,
			"INSERT INTO edition_external_references (edition_id, source, external_id) VALUES ($1, $2, $3)",
			string(e.ID()), ref.Source, ref.ID); err != nil {
			return TranslateError(err)
		}
	}

	return nil
}

func (r *EditionRepository) load(ctx context.Context, exec querier, id domain.EditionID) (*domain.Edition, error) {
	var workID, languageTag string
	var isbn *string
	var publisher string
	var publicationYear *int
	err := exec.QueryRow(ctx,
		"SELECT work_id, language, isbn, publisher, publication_year FROM editions WHERE id = $1",
		string(id),
	).Scan(&workID, &languageTag, &isbn, &publisher, &publicationYear)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "edition not found"}
		}
		return nil, TranslateError(err)
	}

	lang, err := domain.NewLanguage(languageTag)
	if err != nil {
		return nil, TranslateError(err)
	}

	refRows, err := exec.Query(ctx,
		"SELECT source, external_id FROM edition_external_references WHERE edition_id = $1", string(id))
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

	edition, err := domain.NewEdition(id, domain.WorkID(workID), lang, isbn, publisher, publicationYear, refs)
	if err != nil {
		return nil, TranslateError(err)
	}
	return edition, nil
}
