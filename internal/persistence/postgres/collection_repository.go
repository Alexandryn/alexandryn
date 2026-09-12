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

// CollectionRepository is internal/persistence/postgres's
// domain.CollectionRepository implementation. A Collection
// spans two physical tables (collections, collection_members); Save
// replaces every member row for this Collection's id on each call, the
// same replace-on-Save shape as WorkRepository/AuthorRepository/
// EditionRepository. Collection.AddMember/RemoveMember are exported
// (unlike Work's mergedInto/contains, which have no public setter), so
// reads reconstruct a Collection via NewCollection plus repeated
// AddMember calls rather than needing a Rehydrate* path.
type CollectionRepository struct {
	pool *pgxpool.Pool
}

func NewCollectionRepository(pool *pgxpool.Pool) *CollectionRepository {
	return &CollectionRepository{pool: pool}
}

var _ domain.CollectionRepository = (*CollectionRepository)(nil)

func (r *CollectionRepository) FindByID(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID) (*domain.Collection, error) {
	exec := executorFrom(ctx, r.pool)

	var name string
	err := exec.QueryRow(ctx, `SELECT name FROM collections WHERE id = $1 AND library_id = $2`, string(id), string(libraryID)).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "collection not found"}
		}
		return nil, TranslateError(err)
	}

	c, err := domain.NewCollection(id, name)
	if err != nil {
		return nil, TranslateError(err)
	}

	rows, err := exec.Query(ctx,
		`SELECT work_id, added_at FROM collection_members WHERE collection_id = $1`, string(id))
	if err != nil {
		return nil, TranslateError(err)
	}
	for rows.Next() {
		var workID string
		var addedAt time.Time
		if err := rows.Scan(&workID, &addedAt); err != nil {
			rows.Close()
			return nil, TranslateError(err)
		}
		c.AddMember(domain.WorkID(workID), addedAt)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}

	return c, nil
}

func (r *CollectionRepository) Save(ctx context.Context, libraryID domain.LibraryID, c *domain.Collection) error {
	exec := executorFrom(ctx, r.pool)

	// The ON CONFLICT clause updates the row only when it already lives in
	// this library; an id that belongs to another library affects no rows
	// and is reported as NotFound, without disclosing that it exists (#87).
	tag, err := exec.Exec(ctx, `INSERT INTO collections (id, name, library_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name
		WHERE collections.library_id = $3`,
		string(c.ID()), c.Name(), string(libraryID))
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}

	if _, err := exec.Exec(ctx, `DELETE FROM collection_members WHERE collection_id = $1`, string(c.ID())); err != nil {
		return TranslateError(err)
	}
	for _, m := range c.Members() {
		if _, err := exec.Exec(ctx,
			`INSERT INTO collection_members (collection_id, work_id, added_at) VALUES ($1, $2, $3)`,
			string(c.ID()), string(m.WorkID), m.AddedAt); err != nil {
			return TranslateError(err)
		}
	}

	return nil
}

func (r *CollectionRepository) Delete(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID) error {
	exec := executorFrom(ctx, r.pool)

	if _, err := exec.Exec(ctx,
		`DELETE FROM collection_members WHERE collection_id = $1
			AND collection_id IN (SELECT id FROM collections WHERE id = $1 AND library_id = $2)`,
		string(id), string(libraryID)); err != nil {
		return TranslateError(err)
	}
	res, err := exec.Exec(ctx, `DELETE FROM collections WHERE id = $1 AND library_id = $2`, string(id), string(libraryID))
	if err != nil {
		return TranslateError(err)
	}
	if res.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}
	return nil
}

// FindAll returns all collections with their respective member work count, ordered by name ASC.
func (r *CollectionRepository) FindAll(ctx context.Context, libraryID domain.LibraryID) ([]*domain.CollectionSummary, error) {
	exec := executorFrom(ctx, r.pool)

	const query = `SELECT
		c.id,
		c.name,
		COUNT(cm.work_id)::int AS work_count
	FROM collections c
	LEFT JOIN collection_members cm ON cm.collection_id = c.id
	WHERE c.library_id = $1
	GROUP BY c.id, c.name
	ORDER BY c.name ASC`

	rows, err := exec.Query(ctx, query, string(libraryID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.CollectionSummary
	for rows.Next() {
		var (
			id        string
			name      string
			workCount int
		)
		if err := rows.Scan(&id, &name, &workCount); err != nil {
			return nil, TranslateError(err)
		}
		result = append(result, &domain.CollectionSummary{
			ID:        domain.CollectionID(id),
			Name:      name,
			WorkCount: workCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	if result == nil {
		result = []*domain.CollectionSummary{}
	}
	return result, nil
}

type jsonMemberWork struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Subtitle    string              `json:"subtitle"`
	Authors     []string            `json:"authors"`
	IsOwned     bool                `json:"is_owned"`
	Collections []jsonCollectionRef `json:"collections"`
	AddedAt     *time.Time          `json:"added_at"`
}

// FindDetail returns a single collection by ID and all of its member works.
func (r *CollectionRepository) FindDetail(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID) (*domain.CollectionDetail, error) {
	exec := executorFrom(ctx, r.pool)

	const query = `SELECT
		c.id,
		c.name,
		COALESCE((
			SELECT json_agg(
				json_build_object(
					'id', w.id,
					'title', w.title,
					'subtitle', w.subtitle,
					'authors', COALESCE((
						SELECT json_agg(a.name ORDER BY wa.author_id)
						FROM work_authors wa
						JOIN authors a ON a.id = wa.author_id
						WHERE wa.work_id = w.id
					), '[]'::json),
					'is_owned', EXISTS (
						SELECT 1 FROM editions e
						JOIN library_entries le ON le.edition_id = e.id
						WHERE e.work_id = w.id
					),
					'collections', COALESCE((
						SELECT json_agg(json_build_object('id', c2.id, 'name', c2.name, 'added_at', cm2.added_at) ORDER BY cm2.added_at DESC)
						FROM collection_members cm2
						JOIN collections c2 ON c2.id = cm2.collection_id
						WHERE cm2.work_id = w.id
					), '[]'::json),
					'added_at', cm.added_at
				)
				ORDER BY cm.added_at DESC
			)
			FROM collection_members cm
			JOIN works w ON w.id = cm.work_id
			WHERE cm.collection_id = c.id
		), '[]'::json) AS works
	FROM collections c
	WHERE c.id = $1 AND c.library_id = $2`

	var (
		collID    string
		name      string
		worksJSON []byte
	)

	err := exec.QueryRow(ctx, query, string(id), string(libraryID)).Scan(&collID, &name, &worksJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "collection not found"}
		}
		return nil, TranslateError(err)
	}

	var rawWorks []jsonMemberWork
	if len(worksJSON) > 0 {
		if err := json.Unmarshal(worksJSON, &rawWorks); err != nil {
			return nil, TranslateError(err)
		}
	}

	works := make([]*domain.WorkSummary, 0, len(rawWorks))
	for _, rw := range rawWorks {
		authors := rw.Authors
		if authors == nil {
			authors = []string{}
		}
		colls := make([]domain.CollectionRef, 0, len(rw.Collections))
		for _, rc := range rw.Collections {
			addedAt := rc.AddedAt
			colls = append(colls, domain.CollectionRef{
				ID:      domain.CollectionID(rc.ID),
				Name:    rc.Name,
				AddedAt: &addedAt,
			})
		}

		works = append(works, &domain.WorkSummary{
			ID:          domain.WorkID(rw.ID),
			Title:       rw.Title,
			Subtitle:    rw.Subtitle,
			Authors:     authors,
			IsOwned:     rw.IsOwned,
			Collections: colls,
			AddedAt:     rw.AddedAt,
		})
	}

	return &domain.CollectionDetail{
		ID:    domain.CollectionID(collID),
		Name:  name,
		Works: works,
	}, nil
}

// AddMember adds a Work to a Collection idempotently.
func (r *CollectionRepository) AddMember(ctx context.Context, libraryID domain.LibraryID, collectionID domain.CollectionID, workID domain.WorkID, addedAt time.Time) error {
	exec := executorFrom(ctx, r.pool)

	var dummy int
	if err := exec.QueryRow(ctx, `SELECT 1 FROM collections WHERE id = $1 AND library_id = $2`, string(collectionID), string(libraryID)).Scan(&dummy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
		}
		return TranslateError(err)
	}

	if err := exec.QueryRow(ctx, `SELECT 1 FROM works WHERE id = $1`, string(workID)).Scan(&dummy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.Error{Category: domain.NotFound, Message: "work not found"}
		}
		return TranslateError(err)
	}

	const insertSQL = `INSERT INTO collection_members (collection_id, work_id, added_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (collection_id, work_id) DO NOTHING`

	if _, err := exec.Exec(ctx, insertSQL, string(collectionID), string(workID), addedAt); err != nil {
		return TranslateError(err)
	}

	return nil
}

// RemoveMember removes a Work's membership from a Collection.
func (r *CollectionRepository) RemoveMember(ctx context.Context, libraryID domain.LibraryID, collectionID domain.CollectionID, workID domain.WorkID) error {
	exec := executorFrom(ctx, r.pool)

	var dummy int
	if err := exec.QueryRow(ctx, `SELECT 1 FROM collections WHERE id = $1 AND library_id = $2`, string(collectionID), string(libraryID)).Scan(&dummy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
		}
		return TranslateError(err)
	}

	res, err := exec.Exec(ctx, `DELETE FROM collection_members WHERE collection_id = $1 AND work_id = $2`, string(collectionID), string(workID))
	if err != nil {
		return TranslateError(err)
	}
	if res.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "no membership found for that work in this collection"}
	}
	return nil
}

// Rename renames a Collection after validating the new name.
func (r *CollectionRepository) Rename(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID, name string) error {
	if err := domain.ValidateBoundedText("name", name, 100); err != nil {
		return err
	}

	exec := executorFrom(ctx, r.pool)
	res, err := exec.Exec(ctx, `UPDATE collections SET name = $1 WHERE id = $2 AND library_id = $3`, name, string(id), string(libraryID))
	if err != nil {
		return TranslateError(err)
	}
	if res.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}
	return nil
}
