package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// WorkRepository is internal/persistence/postgres's domain.WorkRepository
// implementation (T24, R4). A Work spans five physical tables (works,
// work_authors, work_subjects, work_external_references, and the rows of
// work_contains this Work is the container of, per
// migrations/00002_phase02_schema.sql); Save replaces every child row for
// this Work's id on each call, matching the in-memory fake's own
// overwrite-on-Save semantics (internal/domain/fake_repository_test.go).
type WorkRepository struct {
	pool *pgxpool.Pool
}

func NewWorkRepository(pool *pgxpool.Pool) *WorkRepository {
	return &WorkRepository{pool: pool}
}

var _ domain.WorkRepository = (*WorkRepository)(nil)

func (r *WorkRepository) FindByID(ctx context.Context, id domain.WorkID) (*domain.Work, error) {
	return r.load(ctx, executorFrom(ctx, r.pool), id)
}

// FindMergedInto returns every Work whose merged_into directly equals
// canonical (one hop, not transitive — domain.WorkRepository's own
// contract).
func (r *WorkRepository) FindMergedInto(ctx context.Context, canonical domain.WorkID) ([]*domain.Work, error) {
	exec := executorFrom(ctx, r.pool)

	rows, err := exec.Query(ctx, "SELECT id FROM works WHERE merged_into = $1", string(canonical))
	if err != nil {
		return nil, TranslateError(err)
	}
	var ids []domain.WorkID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, TranslateError(err)
		}
		ids = append(ids, domain.WorkID(id))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	result := make([]*domain.Work, 0, len(ids))
	for _, id := range ids {
		w, err := r.load(ctx, exec, id)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, nil
}

func (r *WorkRepository) Save(ctx context.Context, w *domain.Work) error {
	exec := executorFrom(ctx, r.pool)

	var originalLanguage *string
	if w.OriginalLanguage() != nil {
		s := w.OriginalLanguage().String()
		originalLanguage = &s
	}
	var mergedInto *string
	if w.MergedInto() != nil {
		s := string(*w.MergedInto())
		mergedInto = &s
	}

	_, err := exec.Exec(ctx, `INSERT INTO works (id, title, subtitle, original_language, merged_into)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			subtitle = EXCLUDED.subtitle,
			original_language = EXCLUDED.original_language,
			merged_into = EXCLUDED.merged_into`,
		string(w.ID()), w.Title(), w.Subtitle(), originalLanguage, mergedInto)
	if err != nil {
		return TranslateError(err)
	}

	if _, err := exec.Exec(ctx, "DELETE FROM work_authors WHERE work_id = $1", string(w.ID())); err != nil {
		return TranslateError(err)
	}
	for _, authorID := range w.Authors() {
		if _, err := exec.Exec(ctx,
			"INSERT INTO work_authors (work_id, author_id) VALUES ($1, $2)",
			string(w.ID()), string(authorID)); err != nil {
			return TranslateError(err)
		}
	}

	if _, err := exec.Exec(ctx, "DELETE FROM work_subjects WHERE work_id = $1", string(w.ID())); err != nil {
		return TranslateError(err)
	}
	for _, subject := range w.Subjects() {
		if _, err := exec.Exec(ctx,
			"INSERT INTO work_subjects (work_id, subject) VALUES ($1, $2)",
			string(w.ID()), subject.String()); err != nil {
			return TranslateError(err)
		}
	}

	if _, err := exec.Exec(ctx, "DELETE FROM work_external_references WHERE work_id = $1", string(w.ID())); err != nil {
		return TranslateError(err)
	}
	for _, ref := range w.ExternalReferences() {
		if _, err := exec.Exec(ctx,
			"INSERT INTO work_external_references (work_id, source, external_id) VALUES ($1, $2, $3)",
			string(w.ID()), ref.Source, ref.ID); err != nil {
			return TranslateError(err)
		}
	}

	if _, err := exec.Exec(ctx, "DELETE FROM work_contains WHERE container_work_id = $1", string(w.ID())); err != nil {
		return TranslateError(err)
	}
	for _, containeeID := range w.Contains() {
		if _, err := exec.Exec(ctx,
			"INSERT INTO work_contains (container_work_id, containee_work_id) VALUES ($1, $2)",
			string(w.ID()), string(containeeID)); err != nil {
			return TranslateError(err)
		}
	}

	return nil
}

// load reads one Work by id, including every child-table row, and
// reconstructs it via domain.RehydrateWork (internal/domain/rehydrate.go)
// — the repository's "read from storage" path, distinct from NewWork's
// validating constructor.
func (r *WorkRepository) load(ctx context.Context, exec querier, id domain.WorkID) (*domain.Work, error) {
	var title, subtitle string
	var originalLanguage, mergedInto *string
	err := exec.QueryRow(ctx,
		"SELECT title, subtitle, original_language, merged_into FROM works WHERE id = $1",
		string(id),
	).Scan(&title, &subtitle, &originalLanguage, &mergedInto)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "work not found"}
		}
		return nil, TranslateError(err)
	}

	var lang *domain.Language
	if originalLanguage != nil {
		l, err := domain.NewLanguage(*originalLanguage)
		if err != nil {
			return nil, TranslateError(err)
		}
		lang = &l
	}

	var mergedIntoID *domain.WorkID
	if mergedInto != nil {
		id := domain.WorkID(*mergedInto)
		mergedIntoID = &id
	}

	authorRows, err := exec.Query(ctx, "SELECT author_id FROM work_authors WHERE work_id = $1", string(id))
	if err != nil {
		return nil, TranslateError(err)
	}
	var authors []domain.AuthorID
	for authorRows.Next() {
		var authorID string
		if err := authorRows.Scan(&authorID); err != nil {
			authorRows.Close()
			return nil, TranslateError(err)
		}
		authors = append(authors, domain.AuthorID(authorID))
	}
	if err := authorRows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	subjectRows, err := exec.Query(ctx, "SELECT subject FROM work_subjects WHERE work_id = $1", string(id))
	if err != nil {
		return nil, TranslateError(err)
	}
	var subjects []domain.Subject
	for subjectRows.Next() {
		var value string
		if err := subjectRows.Scan(&value); err != nil {
			subjectRows.Close()
			return nil, TranslateError(err)
		}
		subject, err := domain.NewSubject(value)
		if err != nil {
			return nil, TranslateError(err)
		}
		subjects = append(subjects, subject)
	}
	if err := subjectRows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	refRows, err := exec.Query(ctx,
		"SELECT source, external_id FROM work_external_references WHERE work_id = $1", string(id))
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

	containsRows, err := exec.Query(ctx,
		"SELECT containee_work_id FROM work_contains WHERE container_work_id = $1", string(id))
	if err != nil {
		return nil, TranslateError(err)
	}
	var contains []domain.WorkID
	for containsRows.Next() {
		var containeeID string
		if err := containsRows.Scan(&containeeID); err != nil {
			containsRows.Close()
			return nil, TranslateError(err)
		}
		contains = append(contains, domain.WorkID(containeeID))
	}
	if err := containsRows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	return domain.RehydrateWork(id, title, subtitle, authors, subjects, lang, refs, mergedIntoID, contains), nil
}
