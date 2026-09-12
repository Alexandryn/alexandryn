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

// repositories holds domain repository implementations and services constructed
// against the database connection pool during server startup.
type repositories struct {
	pool               *pgxpool.Pool
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
	users              domain.UserRepository
	credentials        domain.CredentialRepository
	refreshTokens      domain.RefreshTokenRepository
	mfa                domain.MFARepository
	passwordResets     domain.PasswordResetRepository
	libraries          domain.LibraryRepository
	libraryMemberships domain.LibraryMembershipRepository
	libraryInvitations domain.LibraryInvitationRepository
	pairedDevices      domain.PairedDeviceRepository
	networkSettings    domain.NetworkSettingsRepository
	enrolmentGrantJTIs domain.EnrolmentGrantJTIRepository
	mfaTicketJTIs      domain.MFATicketJTIRepository
	networkSweep       *postgres.NetworkSweep
	readingSync        *postgres.ReadingSyncRepository
}

// newRepositories constructs repository implementations against the provided pool.
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
		pool:               pool,
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
		users:              postgres.NewUserRepository(pool),
		credentials:        postgres.NewCredentialRepository(pool),
		refreshTokens:      postgres.NewRefreshTokenRepository(pool),
		mfa:                postgres.NewMFARepository(pool),
		passwordResets:     postgres.NewPasswordResetRepository(pool),
		libraries:          postgres.NewLibraryRepository(pool),
		libraryMemberships: postgres.NewLibraryMembershipRepository(pool),
		libraryInvitations: postgres.NewLibraryInvitationRepository(pool),
		pairedDevices:      postgres.NewPairedDeviceRepository(pool),
		networkSettings:    postgres.NewNetworkSettingsRepository(pool),
		enrolmentGrantJTIs: postgres.NewEnrolmentGrantJTIRepository(pool),
		mfaTicketJTIs:      postgres.NewMFATicketJTIRepository(pool),
		networkSweep:       postgres.NewNetworkSweep(pool),
		readingSync:        postgres.NewReadingSyncRepository(pool),
	}
}
