package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// CollectionRepository is internal/persistence/postgres's
// domain.CollectionRepository implementation (T24, R6). A Collection
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

func (r *CollectionRepository) FindByID(ctx context.Context, id domain.CollectionID) (*domain.Collection, error) {
	exec := executorFrom(ctx, r.pool)

	var name string
	err := exec.QueryRow(ctx, "SELECT name FROM collections WHERE id = $1", string(id)).Scan(&name)
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
		"SELECT work_id, added_at FROM collection_members WHERE collection_id = $1", string(id))
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

func (r *CollectionRepository) Save(ctx context.Context, c *domain.Collection) error {
	exec := executorFrom(ctx, r.pool)

	_, err := exec.Exec(ctx, `INSERT INTO collections (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`,
		string(c.ID()), c.Name())
	if err != nil {
		return TranslateError(err)
	}

	if _, err := exec.Exec(ctx, "DELETE FROM collection_members WHERE collection_id = $1", string(c.ID())); err != nil {
		return TranslateError(err)
	}
	for _, m := range c.Members() {
		if _, err := exec.Exec(ctx,
			"INSERT INTO collection_members (collection_id, work_id, added_at) VALUES ($1, $2, $3)",
			string(c.ID()), string(m.WorkID), m.AddedAt); err != nil {
			return TranslateError(err)
		}
	}

	return nil
}

func (r *CollectionRepository) Delete(ctx context.Context, id domain.CollectionID) error {
	exec := executorFrom(ctx, r.pool)

	if _, err := exec.Exec(ctx, "DELETE FROM collection_members WHERE collection_id = $1", string(id)); err != nil {
		return TranslateError(err)
	}
	if _, err := exec.Exec(ctx, "DELETE FROM collections WHERE id = $1", string(id)); err != nil {
		return TranslateError(err)
	}
	return nil
}
