package main

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// repositories holds every domain repository interface implementation
// (backend-persistence.md FR-2, all 11 aggregates) plus the Transactor
// (ADR 0021), constructed once against the real connection pool as part
// of FR-1 step 6 (backend-service-lifecycle.md), alongside pool
// construction itself — the spec's own State transitions section
// describes step 6 as one step: "construct pool... construct
// repositories," not two. No handler consumes these yet: phase 03
// registers no /api/v1 routes (backend-service-lifecycle.md's own
// Non-goals), so there is nothing here for a future phase's handlers to
// reach for except this struct itself, per FR-2's constructor-injection
// requirement — a struct field, not a package-level global.
type repositories struct {
	works              domain.WorkRepository
	authors            domain.AuthorRepository
	editions           domain.EditionRepository
	libraryEntries     domain.LibraryEntryRepository
	collections        domain.CollectionRepository
	sources            domain.SourceRepository
	sourceOfferings    domain.SourceOfferingRepository
	readingProgress    domain.ReadingProgressRepository
	bookmarks          domain.BookmarkRepository
	highlights         domain.HighlightRepository
	readingPreferences domain.ReadingPreferencesRepository
	transactor         domain.Transactor
}

// newRepositories constructs every T24 repository implementation
// (internal/persistence/postgres) against pool — production's real
// implementation of runDeps.newPool's repository half.
func newRepositories(pool *pgxpool.Pool) *repositories {
	return &repositories{
		works:              postgres.NewWorkRepository(pool),
		authors:            postgres.NewAuthorRepository(pool),
		editions:           postgres.NewEditionRepository(pool),
		libraryEntries:     postgres.NewLibraryEntryRepository(pool),
		collections:        postgres.NewCollectionRepository(pool),
		sources:            postgres.NewSourceRepository(pool),
		sourceOfferings:    postgres.NewSourceOfferingRepository(pool),
		readingProgress:    postgres.NewReadingProgressRepository(pool),
		bookmarks:          postgres.NewBookmarkRepository(pool),
		highlights:         postgres.NewHighlightRepository(pool),
		readingPreferences: postgres.NewReadingPreferencesRepository(pool),
		transactor:         postgres.NewTransactor(pool),
	}
}
