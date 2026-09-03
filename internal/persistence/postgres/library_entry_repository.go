package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// LibraryEntryRepository is internal/persistence/postgres's
// domain.LibraryEntryRepository implementation (T24, R6). A single
// physical table, one row per aggregate — no child tables, no Save
// replace-of-children shape. Save is insert-only (ON CONFLICT DO
// NOTHING): a LibraryEntry has no mutable field once created, and the
// real UNIQUE constraint on library_entries.edition_id (FR-7) is what
// enforces at-most-one-per-Edition, not application logic — a second
// Save for the same edition_id with a different id must lose the race,
// not silently overwrite the first entry the way Work/Author's
// update-in-place Save does.
type LibraryEntryRepository struct {
	pool *pgxpool.Pool
}

func NewLibraryEntryRepository(pool *pgxpool.Pool) *LibraryEntryRepository {
	return &LibraryEntryRepository{pool: pool}
}

var _ domain.LibraryEntryRepository = (*LibraryEntryRepository)(nil)

func (r *LibraryEntryRepository) FindByEdition(ctx context.Context, editionID domain.EditionID) (*domain.LibraryEntry, error) {
	exec := executorFrom(ctx, r.pool)

	var id string
	var addedAt time.Time
	err := exec.QueryRow(ctx,
		"SELECT id, added_at FROM library_entries WHERE edition_id = $1", string(editionID),
	).Scan(&id, &addedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "library entry not found"}
		}
		return nil, TranslateError(err)
	}

	return domain.NewLibraryEntry(domain.LibraryEntryID(id), editionID, addedAt), nil
}

// EditionInLibrary reports whether editionID is owned in libraryID
// (AUDIT-0012-C1). A COALESCE keeps a legacy NULL library_id row
// (pre-multi-library) matching the default library.
func (r *LibraryEntryRepository) EditionInLibrary(ctx context.Context, editionID domain.EditionID, libraryID domain.LibraryID) (bool, error) {
	exec := executorFrom(ctx, r.pool)
	var exists bool
	err := exec.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM library_entries
			WHERE edition_id = $1
			AND COALESCE(library_id::text, '00000000-0000-0000-0000-000000000001') = $2
		)`,
		string(editionID), string(libraryID),
	).Scan(&exists)
	if err != nil {
		return false, TranslateError(err)
	}
	return exists, nil
}

func (r *LibraryEntryRepository) Save(ctx context.Context, e *domain.LibraryEntry) error {
	exec := executorFrom(ctx, r.pool)

	tag, err := exec.Exec(ctx,
		"INSERT INTO library_entries (id, library_id, edition_id, added_at) VALUES ($1, $2, $3, $4) ON CONFLICT (library_id, edition_id) DO NOTHING",
		string(e.ID()), "00000000-0000-0000-0000-000000000001", string(e.EditionID()), e.AddedAt())
	if err != nil {
		tag, err = exec.Exec(ctx,
			"INSERT INTO library_entries (id, edition_id, added_at) VALUES ($1, $2, $3) ON CONFLICT (edition_id) DO NOTHING",
			string(e.ID()), string(e.EditionID()), e.AddedAt())
		if err != nil {
			return TranslateError(err)
		}
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{
			Category: domain.Conflict,
			Message:  "the requested change conflicts with an existing record",
		}
	}
	return nil
}

func (r *LibraryEntryRepository) DeleteByEdition(ctx context.Context, editionID domain.EditionID) error {
	exec := executorFrom(ctx, r.pool)

	if _, err := exec.Exec(ctx, "DELETE FROM library_entries WHERE edition_id = $1", string(editionID)); err != nil {
		return TranslateError(err)
	}
	return nil
}
