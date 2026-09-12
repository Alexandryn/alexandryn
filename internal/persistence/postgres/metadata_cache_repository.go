package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

// MetadataCacheTTL is the 30-day staleness window.
const MetadataCacheTTL = 30 * 24 * time.Hour

// MetadataCacheRepository defines the persistence interface for metadata caching.
type MetadataCacheRepository interface {
	GetWork(ctx context.Context, key string) (*openlibrary.DiscoverWorkDetail, bool, error)
	GetAuthor(ctx context.Context, key string) (*openlibrary.NormalisedAuthor, bool, error)
	SaveWork(ctx context.Context, workKey string, detail *openlibrary.DiscoverWorkDetail) error
	SaveAuthor(ctx context.Context, author *openlibrary.NormalisedAuthor) error
}

// PostgresMetadataCacheRepository implements MetadataCacheRepository backed by PostgreSQL.
type PostgresMetadataCacheRepository struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewMetadataCacheRepository constructs a new PostgresMetadataCacheRepository.
func NewMetadataCacheRepository(pool *pgxpool.Pool) *PostgresMetadataCacheRepository {
	return NewMetadataCacheRepositoryWithClock(pool, time.Now)
}

// NewMetadataCacheRepositoryWithClock allows injecting a custom clock for testing TTL staleness.
func NewMetadataCacheRepositoryWithClock(pool *pgxpool.Pool, now func() time.Time) *PostgresMetadataCacheRepository {
	return &PostgresMetadataCacheRepository{
		pool: pool,
		now:  now,
	}
}

var _ MetadataCacheRepository = (*PostgresMetadataCacheRepository)(nil)

// GetWork fetches a cached work, its editions, and authors if fresh (within 30 days).
func (r *PostgresMetadataCacheRepository) GetWork(ctx context.Context, key string) (*openlibrary.DiscoverWorkDetail, bool, error) {
	key = openlibrary.CleanKey(key)
	if !openlibrary.IsValidWorkKey(key) {
		return nil, false, &domain.Error{Category: domain.InvalidInput, Message: "invalid work key"}
	}

	var (
		title       string
		subtitle    string
		description string
		subjects    []string
		coverURL    *string
		fetchedAt   time.Time
	)

	err := r.pool.QueryRow(ctx,
		`SELECT title, subtitle, description, subjects, cover_url, fetched_at
		 FROM metadata_works WHERE key = $1`, key,
	).Scan(&title, &subtitle, &description, &subjects, &coverURL, &fetchedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, &domain.Error{Category: domain.Internal, Message: "failed to query metadata cache"}
	}

	// Staleness check (30 days)
	if r.now().Sub(fetchedAt) > MetadataCacheTTL {
		return nil, false, nil // Cache miss due to staleness
	}

	// 1. Fetch editions
	rows, err := r.pool.Query(ctx,
		`SELECT key, title, publisher, publish_date, language, cover_url
		 FROM metadata_editions WHERE work_key = $1 ORDER BY fetched_at ASC`, key,
	)
	if err != nil {
		return nil, false, &domain.Error{Category: domain.Internal, Message: "failed to query metadata editions"}
	}
	defer rows.Close()

	var editions []openlibrary.NormalisedEdition
	for rows.Next() {
		var ed openlibrary.NormalisedEdition
		if err := rows.Scan(&ed.OpenLibraryEditionKey, &ed.Title, &ed.Publisher, &ed.PublishDate, &ed.Language, &ed.CoverURL); err != nil {
			return nil, false, &domain.Error{Category: domain.Internal, Message: "failed to scan metadata edition"}
		}
		editions = append(editions, ed)
	}
	if err := rows.Err(); err != nil {
		return nil, false, &domain.Error{Category: domain.Internal, Message: "error iterating metadata editions"}
	}
	if editions == nil {
		editions = []openlibrary.NormalisedEdition{}
	}

	// 2. Fetch authors via join table preserving position
	authorRows, err := r.pool.Query(ctx,
		`SELECT a.key, a.name
		 FROM metadata_authors a
		 JOIN metadata_work_authors mwa ON a.key = mwa.author_key
		 WHERE mwa.work_key = $1
		 ORDER BY mwa.position ASC`, key,
	)
	if err != nil {
		return nil, false, &domain.Error{Category: domain.Internal, Message: "failed to query metadata authors"}
	}
	defer authorRows.Close()

	var authors []openlibrary.NormalisedAuthor
	for authorRows.Next() {
		var k, name string
		if err := authorRows.Scan(&k, &name); err != nil {
			return nil, false, &domain.Error{Category: domain.Internal, Message: "failed to scan metadata author"}
		}
		keyCopy := k
		authors = append(authors, openlibrary.NormalisedAuthor{
			OpenLibraryAuthorKey: &keyCopy,
			Name:                 name,
		})
	}
	if err := authorRows.Err(); err != nil {
		return nil, false, &domain.Error{Category: domain.Internal, Message: "error iterating metadata authors"}
	}
	if authors == nil {
		authors = []openlibrary.NormalisedAuthor{}
	}

	return &openlibrary.DiscoverWorkDetail{
		Work: openlibrary.NormalisedWork{
			Title:       title,
			Subtitle:    subtitle,
			Description: description,
			Subjects:    subjects,
			Authors:     authors,
			CoverURL:    coverURL,
		},
		Editions: editions,
	}, true, nil
}

// GetAuthor fetches a cached author by key if fresh.
func (r *PostgresMetadataCacheRepository) GetAuthor(ctx context.Context, key string) (*openlibrary.NormalisedAuthor, bool, error) {
	key = openlibrary.CleanKey(key)
	if key == "" {
		return nil, false, nil
	}
	var (
		name      string
		fetchedAt time.Time
	)

	err := r.pool.QueryRow(ctx,
		`SELECT name, fetched_at FROM metadata_authors WHERE key = $1`, key,
	).Scan(&name, &fetchedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, &domain.Error{Category: domain.Internal, Message: "failed to query metadata author"}
	}

	if r.now().Sub(fetchedAt) > MetadataCacheTTL {
		return nil, false, nil
	}

	keyCopy := key
	return &openlibrary.NormalisedAuthor{
		OpenLibraryAuthorKey: &keyCopy,
		Name:                 name,
	}, true, nil
}

// SaveWork stores the normalised work, its editions, and authors in a single transaction.
func (r *PostgresMetadataCacheRepository) SaveWork(ctx context.Context, workKey string, detail *openlibrary.DiscoverWorkDetail) error {
	workKey = openlibrary.CleanKey(workKey)
	if !openlibrary.IsValidWorkKey(workKey) {
		return &domain.Error{Category: domain.InvalidInput, Message: "invalid work key"}
	}
	if detail == nil {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to begin transaction"}
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := r.now()

	subjects := detail.Work.Subjects
	if subjects == nil {
		subjects = []string{}
	}

	// 1. Upsert work
	_, err = tx.Exec(ctx,
		`INSERT INTO metadata_works (key, title, subtitle, description, subjects, cover_url, fetched_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (key) DO UPDATE SET
		   title = EXCLUDED.title,
		   subtitle = EXCLUDED.subtitle,
		   description = EXCLUDED.description,
		   subjects = EXCLUDED.subjects,
		   cover_url = EXCLUDED.cover_url,
		   fetched_at = EXCLUDED.fetched_at`,
		workKey,
		detail.Work.Title,
		detail.Work.Subtitle,
		detail.Work.Description,
		subjects,
		detail.Work.CoverURL,
		now,
	)
	if err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to upsert metadata work"}
	}

	// 2. Upsert authors and link in join table
	_, err = tx.Exec(ctx, `DELETE FROM metadata_work_authors WHERE work_key = $1`, workKey)
	if err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to clear existing work authors"}
	}

	for pos, a := range detail.Work.Authors {
		if strings.TrimSpace(a.Name) == "" {
			continue
		}
		var authorKey string
		if a.OpenLibraryAuthorKey != nil && *a.OpenLibraryAuthorKey != "" {
			authorKey = openlibrary.CleanKey(*a.OpenLibraryAuthorKey)
			if authorKey == "" {
				continue
			}
		} else {
			continue
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO metadata_authors (key, name, fetched_at)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (key) DO UPDATE SET
			   name = EXCLUDED.name,
			   fetched_at = EXCLUDED.fetched_at`,
			authorKey, a.Name, now,
		)
		if err != nil {
			return &domain.Error{Category: domain.Internal, Message: "failed to upsert metadata author"}
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO metadata_work_authors (work_key, author_key, position)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (work_key, author_key) DO UPDATE SET position = EXCLUDED.position`,
			workKey, authorKey, pos,
		)
		if err != nil {
			return &domain.Error{Category: domain.Internal, Message: "failed to insert metadata work author link"}
		}
	}

	// 3. Clear existing editions and insert fresh editions
	_, err = tx.Exec(ctx, `DELETE FROM metadata_editions WHERE work_key = $1`, workKey)
	if err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to clear existing work editions"}
	}

	for _, ed := range detail.Editions {
		edKey := openlibrary.CleanKey(ed.OpenLibraryEditionKey)
		if edKey == "" {
			continue
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO metadata_editions (key, work_key, title, publisher, publish_date, language, cover_url, fetched_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (key) DO UPDATE SET
			   work_key = EXCLUDED.work_key,
			   title = EXCLUDED.title,
			   publisher = EXCLUDED.publisher,
			   publish_date = EXCLUDED.publish_date,
			   language = EXCLUDED.language,
			   cover_url = EXCLUDED.cover_url,
			   fetched_at = EXCLUDED.fetched_at`,
			edKey, workKey, ed.Title, ed.Publisher, ed.PublishDate, ed.Language, ed.CoverURL, now,
		)
		if err != nil {
			return &domain.Error{Category: domain.Internal, Message: "failed to upsert metadata edition"}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to commit metadata transaction"}
	}

	return nil
}

// SaveAuthor stores or refreshes a single author in the cache.
func (r *PostgresMetadataCacheRepository) SaveAuthor(ctx context.Context, author *openlibrary.NormalisedAuthor) error {
	if author == nil || author.OpenLibraryAuthorKey == nil || *author.OpenLibraryAuthorKey == "" || strings.TrimSpace(author.Name) == "" {
		return nil
	}
	key := openlibrary.CleanKey(*author.OpenLibraryAuthorKey)
	if key == "" {
		return nil
	}
	now := r.now()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO metadata_authors (key, name, fetched_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (key) DO UPDATE SET
		   name = EXCLUDED.name,
		   fetched_at = EXCLUDED.fetched_at`,
		key, strings.TrimSpace(author.Name), now,
	)
	if err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to upsert metadata author"}
	}
	return nil
}
