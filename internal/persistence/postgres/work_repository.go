package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// WorkRepository is internal/persistence/postgres's domain.WorkRepository
// implementation. A Work spans five physical tables (works,
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

type jsonCollectionRef struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	AddedAt time.Time `json:"added_at"`
}

type jsonOwnedEdition struct {
	ID              string     `json:"id"`
	Language        string     `json:"language"`
	ISBN            *string    `json:"isbn"`
	Publisher       string     `json:"publisher"`
	PublicationYear *int       `json:"publication_year"`
	AddedAt         *time.Time `json:"added_at"`
	Formats         []string   `json:"formats"`
}

// QueryLibrary performs a single-query paginated fetch of works matching
// the given filter, search, sort, and cursor parameters.
func (r *WorkRepository) QueryLibrary(ctx context.Context, q domain.LibraryQuery) (*domain.LibraryPage, error) {
	exec := executorFrom(ctx, r.pool)

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}

	filter := q.Filter
	if filter == "" {
		filter = domain.FilterAll
	}

	sort := q.Sort
	if sort == "" {
		sort = domain.SortAddedAt
	}

	var cursorAddedAt time.Time
	var cursorTitle string
	var cursorID string
	var hasCursor bool

	if q.Cursor != "" {
		if sort == domain.SortTitle {
			t, id, err := domain.DecodeTitleCursor(q.Cursor)
			if err != nil {
				return nil, err
			}
			cursorTitle = t
			cursorID = string(id)
			hasCursor = true
		} else {
			t, id, err := domain.DecodeAddedAtCursor(q.Cursor)
			if err != nil {
				return nil, err
			}
			cursorAddedAt = t
			cursorID = string(id)
			hasCursor = true
		}
	}

	const workPoolCTE = `WITH work_pool AS (
			SELECT
				w.id,
				w.title,
				w.subtitle,
				EXISTS (
					SELECT 1 FROM editions e
					JOIN library_entries le ON le.edition_id = e.id AND le.library_id = $3
					WHERE e.work_id = w.id
				) AS is_owned,
				COALESCE(
					(SELECT min(le.added_at) FROM editions e JOIN library_entries le ON le.edition_id = e.id AND le.library_id = $3 WHERE e.work_id = w.id),
					(SELECT max(cm.added_at) FROM collection_members cm JOIN collections c ON c.id = cm.collection_id AND c.library_id = $3 WHERE cm.work_id = w.id)
				) AS effective_added_at
			FROM works w
			WHERE
				(
					($1 = 'owned' AND EXISTS (
						SELECT 1 FROM editions e
						JOIN library_entries le ON le.edition_id = e.id AND le.library_id = $3
						WHERE e.work_id = w.id
					))
					OR ($1 = 'wanted' AND EXISTS (
						SELECT 1 FROM collection_members cm
						JOIN collections c ON c.id = cm.collection_id AND c.library_id = $3
						WHERE cm.work_id = w.id
					) AND NOT EXISTS (
						SELECT 1 FROM editions e
						JOIN library_entries le ON le.edition_id = e.id AND le.library_id = $3
						WHERE e.work_id = w.id
					))
					OR ($1 = 'all' AND (
						EXISTS (
							SELECT 1 FROM editions e
							JOIN library_entries le ON le.edition_id = e.id AND le.library_id = $3
							WHERE e.work_id = w.id
						) OR EXISTS (
							SELECT 1 FROM collection_members cm
							JOIN collections c ON c.id = cm.collection_id AND c.library_id = $3
							WHERE cm.work_id = w.id
						)
					))
				)
				AND (
					$2 = ''
					OR w.search_vector @@ plainto_tsquery('simple', $2)
					OR EXISTS (
						SELECT 1 FROM work_authors wa
						JOIN authors a ON a.id = wa.author_id
						WHERE wa.work_id = w.id AND a.name_vector @@ plainto_tsquery('simple', $2)
					)
				)
		)
		SELECT
			p.id,
			p.title,
			p.subtitle,
			p.is_owned,
			p.effective_added_at,
			COALESCE((
				SELECT json_agg(a.name ORDER BY wa.author_id)
				FROM work_authors wa
				JOIN authors a ON a.id = wa.author_id
				WHERE wa.work_id = p.id
			), '[]'::json) AS authors,
			COALESCE((
				SELECT json_agg(json_build_object('id', c.id, 'name', c.name, 'added_at', cm.added_at) ORDER BY cm.added_at DESC)
				FROM collection_members cm
				JOIN collections c ON c.id = cm.collection_id
				WHERE cm.work_id = p.id AND c.library_id = $3
			), '[]'::json) AS collections
		FROM work_pool p
		`

	// The four variants share workPoolCTE (every library_entries and
	// collection reference scoped to $3, the active library) and differ
	// only in the ORDER BY / cursor predicate / LIMIT tail.
	// Cursor params, when present: $4 primary sort key, $5 id, $6 limit.
	const (
		queryAddedAtNoCursor = workPoolCTE + `
			ORDER BY p.effective_added_at DESC, p.id DESC
			LIMIT $4`
		queryAddedAtWithCursor = workPoolCTE + `
			WHERE (p.effective_added_at < $4 OR (p.effective_added_at = $4 AND p.id < $5))
			ORDER BY p.effective_added_at DESC, p.id DESC
			LIMIT $6`
		queryTitleNoCursor = workPoolCTE + `
			ORDER BY p.title ASC, p.id ASC
			LIMIT $4`
		queryTitleWithCursor = workPoolCTE + `
			WHERE (p.title > $4 OR (p.title = $4 AND p.id > $5))
			ORDER BY p.title ASC, p.id ASC
			LIMIT $6`
	)

	libraryID := string(q.LibraryID)
	if libraryID == "" {
		libraryID = string(domain.DefaultLibraryID)
	}

	var (
		queryToExec string
		args        []any
	)

	if sort == domain.SortTitle {
		if hasCursor {
			queryToExec = queryTitleWithCursor
			args = []any{string(filter), q.Q, libraryID, cursorTitle, cursorID, limit + 1}
		} else {
			queryToExec = queryTitleNoCursor
			args = []any{string(filter), q.Q, libraryID, limit + 1}
		}
	} else {
		if hasCursor {
			queryToExec = queryAddedAtWithCursor
			args = []any{string(filter), q.Q, libraryID, cursorAddedAt, cursorID, limit + 1}
		} else {
			queryToExec = queryAddedAtNoCursor
			args = []any{string(filter), q.Q, libraryID, limit + 1}
		}
	}

	rows, err := exec.Query(ctx, queryToExec, args...)

	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	type workRow struct {
		id               string
		title            string
		subtitle         string
		isOwned          bool
		effectiveAddedAt *time.Time
		authorsJSON      []byte
		collectionsJSON  []byte
	}

	var fetched []workRow
	for rows.Next() {
		var row workRow
		if err := rows.Scan(
			&row.id,
			&row.title,
			&row.subtitle,
			&row.isOwned,
			&row.effectiveAddedAt,
			&row.authorsJSON,
			&row.collectionsJSON,
		); err != nil {
			return nil, TranslateError(err)
		}
		fetched = append(fetched, row)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	hasMore := len(fetched) > limit
	if hasMore {
		fetched = fetched[:limit]
	}

	resultWorks := make([]*domain.WorkSummary, 0, len(fetched))
	for _, row := range fetched {
		var authors []string
		if len(row.authorsJSON) > 0 {
			if err := json.Unmarshal(row.authorsJSON, &authors); err != nil {
				return nil, TranslateError(err)
			}
		}
		if authors == nil {
			authors = []string{}
		}

		var rawColls []jsonCollectionRef
		if len(row.collectionsJSON) > 0 {
			if err := json.Unmarshal(row.collectionsJSON, &rawColls); err != nil {
				return nil, TranslateError(err)
			}
		}

		collections := make([]domain.CollectionRef, 0, len(rawColls))
		for _, c := range rawColls {
			addedAt := c.AddedAt
			collections = append(collections, domain.CollectionRef{
				ID:      domain.CollectionID(c.ID),
				Name:    c.Name,
				AddedAt: &addedAt,
			})
		}

		resultWorks = append(resultWorks, &domain.WorkSummary{
			ID:          domain.WorkID(row.id),
			Title:       row.title,
			Subtitle:    row.subtitle,
			Authors:     authors,
			IsOwned:     row.isOwned,
			Collections: collections,
			AddedAt:     row.effectiveAddedAt,
		})
	}

	var nextCursor string
	if hasMore && len(resultWorks) > 0 {
		last := resultWorks[len(resultWorks)-1]
		if sort == domain.SortTitle {
			nextCursor = domain.EncodeTitleCursor(last.Title, last.ID)
		} else {
			if last.AddedAt != nil {
				nextCursor = domain.EncodeAddedAtCursor(*last.AddedAt, last.ID)
			} else {
				nextCursor = domain.EncodeAddedAtCursor(time.Time{}, last.ID)
			}
		}
	}

	return &domain.LibraryPage{
		Works:      resultWorks,
		NextCursor: nextCursor,
	}, nil
}

// FindWorkDetail loads one Work's detail including owned editions and
// collection memberships.
func (r *WorkRepository) FindWorkDetail(ctx context.Context, id domain.WorkID, libraryID domain.LibraryID) (*domain.WorkDetail, error) {
	exec := executorFrom(ctx, r.pool)

	lib := string(libraryID)
	if lib == "" {
		lib = string(domain.DefaultLibraryID)
	}

	// $2 is the active library. Owned editions and collection memberships
	// are scoped to it, and a work with neither in this library is a 404 —
	// preventing cross-library holdings disclosure.
	query := `SELECT
		w.id,
		w.title,
		w.subtitle,
		w.original_language,
		COALESCE((
			SELECT json_agg(a.name ORDER BY wa.author_id)
			FROM work_authors wa
			JOIN authors a ON a.id = wa.author_id
			WHERE wa.work_id = w.id
		), '[]'::json) AS authors,
		COALESCE((
			SELECT json_agg(s.subject ORDER BY s.subject)
			FROM work_subjects s
			WHERE s.work_id = w.id
		), '[]'::json) AS subjects,
		COALESCE((
			SELECT json_agg(
				json_build_object(
					'id', e.id,
					'language', e.language,
					'isbn', e.isbn,
					'publisher', e.publisher,
					'publication_year', e.publication_year,
					'added_at', le.added_at,
					'formats', COALESCE((
						SELECT json_agg(DISTINCT so.file_reference_format ORDER BY so.file_reference_format)
						FROM source_offerings so
						WHERE so.edition_id = e.id
					), '[]'::json)
				)
				ORDER BY le.added_at DESC
			)
			FROM editions e
			JOIN library_entries le ON le.edition_id = e.id AND le.library_id = $2
			WHERE e.work_id = w.id
		), '[]'::json) AS owned_editions,
		COALESCE((
			SELECT json_agg(
				json_build_object(
					'id', c.id,
					'name', c.name,
					'added_at', cm.added_at
				)
				ORDER BY cm.added_at DESC
			)
			FROM collection_members cm
			JOIN collections c ON c.id = cm.collection_id AND c.library_id = $2
			WHERE cm.work_id = w.id
		), '[]'::json) AS collections
	FROM works w
	WHERE w.id = $1
		AND (
			EXISTS (
				SELECT 1 FROM editions e
				JOIN library_entries le ON le.edition_id = e.id AND le.library_id = $2
				WHERE e.work_id = w.id
			)
			OR EXISTS (
				SELECT 1 FROM collection_members cm
				JOIN collections c ON c.id = cm.collection_id AND c.library_id = $2
				WHERE cm.work_id = w.id
			)
		)`

	var workID, title, subtitle string
	var originalLanguage *string
	var authorsJSON, subjectsJSON, editionsJSON, collectionsJSON []byte

	err := exec.QueryRow(ctx, query, string(id), lib).Scan(
		&workID,
		&title,
		&subtitle,
		&originalLanguage,
		&authorsJSON,
		&subjectsJSON,
		&editionsJSON,
		&collectionsJSON,
	)
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

	var authors []string
	if len(authorsJSON) > 0 {
		if err := json.Unmarshal(authorsJSON, &authors); err != nil {
			return nil, TranslateError(err)
		}
	}
	if authors == nil {
		authors = []string{}
	}

	var subjects []string
	if len(subjectsJSON) > 0 {
		if err := json.Unmarshal(subjectsJSON, &subjects); err != nil {
			return nil, TranslateError(err)
		}
	}
	if subjects == nil {
		subjects = []string{}
	}

	var rawEditions []jsonOwnedEdition
	if len(editionsJSON) > 0 {
		if err := json.Unmarshal(editionsJSON, &rawEditions); err != nil {
			return nil, TranslateError(err)
		}
	}

	ownedEditions := make([]domain.OwnedEdition, 0, len(rawEditions))
	for _, e := range rawEditions {
		formats := e.Formats
		if formats == nil {
			formats = []string{}
		}
		ownedEditions = append(ownedEditions, domain.OwnedEdition{
			ID:              domain.EditionID(e.ID),
			Language:        e.Language,
			ISBN:            e.ISBN,
			Publisher:       e.Publisher,
			PublicationYear: e.PublicationYear,
			AddedAt:         e.AddedAt,
			Formats:         formats,
		})
	}

	var rawColls []jsonCollectionRef
	if len(collectionsJSON) > 0 {
		if err := json.Unmarshal(collectionsJSON, &rawColls); err != nil {
			return nil, TranslateError(err)
		}
	}

	collections := make([]domain.CollectionRef, 0, len(rawColls))
	for _, c := range rawColls {
		addedAt := c.AddedAt
		collections = append(collections, domain.CollectionRef{
			ID:      domain.CollectionID(c.ID),
			Name:    c.Name,
			AddedAt: &addedAt,
		})
	}

	return &domain.WorkDetail{
		ID:               domain.WorkID(workID),
		Title:            title,
		Subtitle:         subtitle,
		Authors:          authors,
		Subjects:         subjects,
		OriginalLanguage: lang,
		OwnedEditions:    ownedEditions,
		Collections:      collections,
	}, nil
}
