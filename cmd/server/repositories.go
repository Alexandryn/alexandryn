package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/idgen"
	"github.com/Alexandryn/alexandryn/internal/importer"
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
	metadataCache      postgres.MetadataCacheRepository
	coverCache         postgres.CoverCacheRepository
	sourceRecords      *postgres.SourceRecordRepository
	sourceRemoval      *domain.SourceRemovalService
	importCandidates   *postgres.ImportCandidateRepository
	importerService    *importer.Service
	readingExport      *postgres.ReadingExportRepository
}

// newRepositories constructs every T24 repository implementation
// (internal/persistence/postgres) against pool — production's real
// implementation of runDeps.newPool's repository half.
func newRepositories(pool *pgxpool.Pool, loggers ...*slog.Logger) *repositories {
	var l *slog.Logger
	if len(loggers) > 0 {
		l = loggers[0]
	}
	coversDir := ""
	if userCache, err := os.UserCacheDir(); err == nil && userCache != "" {
		coversDir = filepath.Join(userCache, "alexandryn", "covers")
	} else {
		coversDir = filepath.Join(os.TempDir(), "alexandryn-covers")
	}
	coverCacheRepo, _ := postgres.NewCoverCacheRepository(pool, coversDir, l)

	sourceRepo := postgres.NewSourceRepository(pool)
	sourceOfferingRepo := postgres.NewSourceOfferingRepository(pool)
	transactor := postgres.NewTransactor(pool)
	sourceRemovalSvc := domain.NewSourceRemovalService(sourceRepo, sourceOfferingRepo, transactor)

	workRepo := postgres.NewWorkRepository(pool)
	authorRepo := postgres.NewAuthorRepository(pool)
	editionRepo := postgres.NewEditionRepository(pool)
	libraryEntryRepo := postgres.NewLibraryEntryRepository(pool)
	candRepo := postgres.NewImportCandidateRepository(pool)
	idGen := idgen.New()
	librarySvc := domain.NewLibraryService(editionRepo, libraryEntryRepo)

	importerSvc := importer.NewService(
		workRepo,
		editionRepo,
		authorRepo,
		sourceOfferingRepo,
		libraryEntryRepo,
		candRepo,
		librarySvc,
		transactor,
		idGen,
	)

	return &repositories{
		works:              workRepo,
		authors:            authorRepo,
		editions:           editionRepo,
		libraryEntries:     libraryEntryRepo,
		collections:        postgres.NewCollectionRepository(pool),
		sources:            sourceRepo,
		sourceOfferings:    sourceOfferingRepo,
		readingProgress:    postgres.NewReadingProgressRepository(pool),
		bookmarks:          postgres.NewBookmarkRepository(pool),
		highlights:         postgres.NewHighlightRepository(pool),
		readingPreferences: postgres.NewReadingPreferencesRepository(pool),
		transactor:         transactor,
		metadataCache:      postgres.NewMetadataCacheRepository(pool),
		coverCache:         coverCacheRepo,
		sourceRecords:      postgres.NewSourceRecordRepository(pool),
		sourceRemoval:      sourceRemovalSvc,
		importCandidates:   candRepo,
		importerService:    importerSvc,
		readingExport:      postgres.NewReadingExportRepository(pool),
	}
}
