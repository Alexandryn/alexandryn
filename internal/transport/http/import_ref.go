package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources/local"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources/opds"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

type importRefs struct {
	candidates atomic.Pointer[postgres.ImportCandidateRepository]
	service    atomic.Pointer[importer.Service]
	discovery  atomic.Pointer[importer.DiscoveryCoordinator]
}

// SetImportCandidateRepository stores the import candidate repository.
func (r *PoolRef) SetImportCandidateRepository(repo *postgres.ImportCandidateRepository) {
	r.imports.candidates.Store(repo)
}

// GetImportCandidateRepository returns the import candidate repository, if set.
func (r *PoolRef) GetImportCandidateRepository() (*postgres.ImportCandidateRepository, bool) {
	v := r.imports.candidates.Load()
	return v, v != nil
}

// SetImporterService stores the import domain service.
func (r *PoolRef) SetImporterService(svc *importer.Service) {
	r.imports.service.Store(svc)
}

// GetImporterService returns the importer service, if set.
func (r *PoolRef) GetImporterService() (*importer.Service, bool) {
	v := r.imports.service.Load()
	return v, v != nil
}

// SetDiscoveryCoordinator stores the discovery coordinator.
func (r *PoolRef) SetDiscoveryCoordinator(coord *importer.DiscoveryCoordinator) {
	r.imports.discovery.Store(coord)
}

// GetDiscoveryCoordinator returns the discovery coordinator, if set.
func (r *PoolRef) GetDiscoveryCoordinator() (*importer.DiscoveryCoordinator, bool) {
	v := r.imports.discovery.Load()
	return v, v != nil
}

// LazyImportCandidateRepository delegates candidate queries to the repository on PoolRef.
type LazyImportCandidateRepository struct {
	ref *PoolRef
}

func NewLazyImportCandidateRepository(ref *PoolRef) *LazyImportCandidateRepository {
	return &LazyImportCandidateRepository{ref: ref}
}

func (l *LazyImportCandidateRepository) get() (*postgres.ImportCandidateRepository, error) {
	repo, ok := l.ref.GetImportCandidateRepository()
	if !ok {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return repo, nil
}

func (l *LazyImportCandidateRepository) Get(ctx context.Context, id string) (postgres.ImportCandidateRecord, error) {
	repo, err := l.get()
	if err != nil {
		return postgres.ImportCandidateRecord{}, err
	}
	return repo.Get(ctx, id)
}

func (l *LazyImportCandidateRepository) List(ctx context.Context, sourceID *string, status *string) ([]postgres.ImportCandidateRecord, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.List(ctx, sourceID, status)
}

// LazyDiscoveryRunner delegates discovery runs to the DiscoveryCoordinator on PoolRef.
type LazyDiscoveryRunner struct {
	ref *PoolRef
}

func NewLazyDiscoveryRunner(ref *PoolRef) *LazyDiscoveryRunner {
	return &LazyDiscoveryRunner{ref: ref}
}

func (l *LazyDiscoveryRunner) Discover(ctx context.Context, sourceID string, now time.Time) (importer.DiscoverResult, error) {
	coord, ok := l.ref.GetDiscoveryCoordinator()
	if !ok {
		return importer.DiscoverResult{}, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return coord.Discover(ctx, sourceID, now)
}

// LazyImporterService delegates import confirmation/rejection to the importer.Service on PoolRef.
type LazyImporterService struct {
	ref *PoolRef
}

func NewLazyImporterService(ref *PoolRef) *LazyImporterService {
	return &LazyImporterService{ref: ref}
}

func (l *LazyImporterService) get() (*importer.Service, error) {
	svc, ok := l.ref.GetImporterService()
	if !ok {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return svc, nil
}

func (l *LazyImporterService) ConfirmAttachExisting(ctx context.Context, candidateID string, editionID string, now time.Time) error {
	svc, err := l.get()
	if err != nil {
		return err
	}
	return svc.ConfirmAttachExisting(ctx, candidateID, editionID, now)
}

func (l *LazyImporterService) ConfirmCreateNew(ctx context.Context, candidateID string, meta extract.ExtractedMetadata, now time.Time) error {
	svc, err := l.get()
	if err != nil {
		return err
	}
	return svc.ConfirmCreateNew(ctx, candidateID, meta, now)
}

func (l *LazyImporterService) ConfirmOpenLibraryMatch(ctx context.Context, candidateID string, openLibraryWorkKey string, meta extract.ExtractedMetadata, olDetail *openlibrary.DiscoverWorkDetail, now time.Time) error {
	svc, err := l.get()
	if err != nil {
		return err
	}
	return svc.ConfirmOpenLibraryMatch(ctx, candidateID, openLibraryWorkKey, meta, olDetail, now)
}

func (l *LazyImporterService) Reject(ctx context.Context, candidateID string, now time.Time) error {
	svc, err := l.get()
	if err != nil {
		return err
	}
	return svc.Reject(ctx, candidateID, now)
}

// SourceProviderResolver builds sources.Provider instances on demand for import extractors and discovery.
type SourceProviderResolver struct {
	repo    SourceRecordRepository
	poolRef *PoolRef
	logger  *slog.Logger
}

func NewSourceProviderResolver(repo SourceRecordRepository, poolRef *PoolRef, logger *slog.Logger) *SourceProviderResolver {
	return &SourceProviderResolver{
		repo:    repo,
		poolRef: poolRef,
		logger:  logger,
	}
}

func (r *SourceProviderResolver) ProviderFor(ctx context.Context, sourceID string) (sources.Provider, error) {
	rec, err := r.repo.Get(ctx, sourceID)
	if err != nil {
		return nil, err
	}

	sc, ok := r.poolRef.GetSourceCrypto()
	if !ok || sc.Codec == nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "source crypto not ready"}
	}

	if rec.Kind == string(sources.KindLocalFolder) {
		return local.New(sourceID, rec.ConfigBasePath, sc.Codec, r.logger)
	}

	if rec.Kind == string(sources.KindOPDS) {
		var cred sources.Credential
		if rec.HasCredential() && sc.Encryptor != nil {
			pt, err := sc.Encryptor.Decrypt(rec.CredentialCiphertext, rec.CredentialNonce)
			if err == nil {
				var credInput wireSourceCredentialInput
				if err := json.Unmarshal(pt, &credInput); err == nil {
					if c, err := sources.NewCredential(credInput.Username, credInput.Password); err == nil {
						cred = c
					}
				}
			}
		}
		return opds.New(opds.Config{
			SourceID:              sourceID,
			BaseURL:               rec.ConfigBaseURL,
			Credential:            cred,
			HasCredential:         rec.HasCredential(),
			SearchTemplate:        rec.SearchLinkURL,
			Codec:                 sc.Codec,
			Logger:                r.logger,
			AllowPrivateAddresses: r.poolRef.SourceAllowPrivateAddresses(),
		}), nil
	}

	return nil, &domain.Error{Category: domain.InvalidInput, Message: fmt.Sprintf("unsupported source kind %q", rec.Kind)}
}

func (r *SourceProviderResolver) Resolve(ctx context.Context, sourceID string, fileRef domain.FileReference) (io.ReadCloser, error) {
	p, err := r.ProviderFor(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	return p.Resolve(ctx, fileRef)
}

func (r *SourceProviderResolver) ListSource(ctx context.Context, sourceID string, cursor string, limit int) (sources.CandidatePage, error) {
	p, err := r.ProviderFor(ctx, sourceID)
	if err != nil {
		return sources.CandidatePage{}, err
	}
	return p.List(ctx, cursor, limit)
}

// SourceCheckerAdapter adapts domain.SourceRepository to importer.SourceChecker.
type SourceCheckerAdapter struct {
	repo domain.SourceRepository
}

func NewSourceCheckerAdapter(repo domain.SourceRepository) *SourceCheckerAdapter {
	return &SourceCheckerAdapter{repo: repo}
}

func (a *SourceCheckerAdapter) GetSource(ctx context.Context, sourceID string) (*domain.Source, error) {
	return a.repo.FindByID(ctx, domain.SourceID(sourceID))
}

// JobQueueEnqueuer adapts *jobs.Queue to importer.JobEnqueuer.
type JobQueueEnqueuer struct {
	queue *jobs.Queue
}

func NewJobQueueEnqueuer(q *jobs.Queue) *JobQueueEnqueuer {
	return &JobQueueEnqueuer{queue: q}
}

func (e *JobQueueEnqueuer) EnqueueImportJob(ctx context.Context, payload importer.ImportJobPayload) (string, error) {
	jobID, err := e.queue.Enqueue(ctx, "import", payload)
	if err != nil {
		return "", err
	}
	return string(jobID), nil
}
