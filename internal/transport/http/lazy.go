package http

import (
	"context"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// LazyWorkRepository delegates to the domain.WorkRepository stored on
// PoolRef once set, returning domain.Unavailable when not yet ready.
type LazyWorkRepository struct {
	ref *PoolRef
}

// NewLazyWorkRepository returns a domain.WorkRepository wrapping ref.
func NewLazyWorkRepository(ref *PoolRef) domain.WorkRepository {
	return &LazyWorkRepository{ref: ref}
}

var _ domain.WorkRepository = (*LazyWorkRepository)(nil)

func (l *LazyWorkRepository) get() (domain.WorkRepository, error) {
	repo, ok := l.ref.GetWorkRepository()
	if !ok {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return repo, nil
}

func (l *LazyWorkRepository) FindByID(ctx context.Context, id domain.WorkID) (*domain.Work, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.FindByID(ctx, id)
}

func (l *LazyWorkRepository) FindMergedInto(ctx context.Context, canonical domain.WorkID) ([]*domain.Work, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.FindMergedInto(ctx, canonical)
}

func (l *LazyWorkRepository) Save(ctx context.Context, w *domain.Work) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.Save(ctx, w)
}

func (l *LazyWorkRepository) QueryLibrary(ctx context.Context, q domain.LibraryQuery) (*domain.LibraryPage, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.QueryLibrary(ctx, q)
}

func (l *LazyWorkRepository) FindWorkDetail(ctx context.Context, id domain.WorkID, libraryID domain.LibraryID) (*domain.WorkDetail, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.FindWorkDetail(ctx, id, libraryID)
}

// LazyCollectionRepository delegates to the domain.CollectionRepository stored on
// PoolRef once set, returning domain.Unavailable when not yet ready.
type LazyCollectionRepository struct {
	ref *PoolRef
}

// NewLazyCollectionRepository returns a domain.CollectionRepository wrapping ref.
func NewLazyCollectionRepository(ref *PoolRef) domain.CollectionRepository {
	return &LazyCollectionRepository{ref: ref}
}

var _ domain.CollectionRepository = (*LazyCollectionRepository)(nil)

func (l *LazyCollectionRepository) get() (domain.CollectionRepository, error) {
	repo, ok := l.ref.GetCollectionRepository()
	if !ok {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return repo, nil
}

func (l *LazyCollectionRepository) FindByID(ctx context.Context, id domain.CollectionID) (*domain.Collection, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.FindByID(ctx, id)
}

func (l *LazyCollectionRepository) Save(ctx context.Context, c *domain.Collection) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.Save(ctx, c)
}

func (l *LazyCollectionRepository) Delete(ctx context.Context, id domain.CollectionID) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.Delete(ctx, id)
}

func (l *LazyCollectionRepository) FindAll(ctx context.Context) ([]*domain.CollectionSummary, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.FindAll(ctx)
}

func (l *LazyCollectionRepository) FindDetail(ctx context.Context, id domain.CollectionID) (*domain.CollectionDetail, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.FindDetail(ctx, id)
}

func (l *LazyCollectionRepository) AddMember(ctx context.Context, collectionID domain.CollectionID, workID domain.WorkID, addedAt time.Time) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.AddMember(ctx, collectionID, workID, addedAt)
}

func (l *LazyCollectionRepository) RemoveMember(ctx context.Context, collectionID domain.CollectionID, workID domain.WorkID) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.RemoveMember(ctx, collectionID, workID)
}

func (l *LazyCollectionRepository) Rename(ctx context.Context, id domain.CollectionID, name string) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.Rename(ctx, id, name)
}

// LazyMetadataCacheRepository delegates to postgres.MetadataCacheRepository stored on PoolRef.
type LazyMetadataCacheRepository struct {
	ref *PoolRef
}

// NewLazyMetadataCacheRepository returns a postgres.MetadataCacheRepository wrapping ref.
func NewLazyMetadataCacheRepository(ref *PoolRef) postgres.MetadataCacheRepository {
	return &LazyMetadataCacheRepository{ref: ref}
}

var _ postgres.MetadataCacheRepository = (*LazyMetadataCacheRepository)(nil)

func (l *LazyMetadataCacheRepository) get() (postgres.MetadataCacheRepository, error) {
	repo, ok := l.ref.GetMetadataCacheRepository()
	if !ok {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return repo, nil
}

func (l *LazyMetadataCacheRepository) GetWork(ctx context.Context, key string) (*openlibrary.DiscoverWorkDetail, bool, error) {
	repo, err := l.get()
	if err != nil {
		return nil, false, err
	}
	return repo.GetWork(ctx, key)
}

func (l *LazyMetadataCacheRepository) GetAuthor(ctx context.Context, key string) (*openlibrary.NormalisedAuthor, bool, error) {
	repo, err := l.get()
	if err != nil {
		return nil, false, err
	}
	return repo.GetAuthor(ctx, key)
}

func (l *LazyMetadataCacheRepository) SaveWork(ctx context.Context, workKey string, detail *openlibrary.DiscoverWorkDetail) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.SaveWork(ctx, workKey, detail)
}

func (l *LazyMetadataCacheRepository) SaveAuthor(ctx context.Context, author *openlibrary.NormalisedAuthor) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.SaveAuthor(ctx, author)
}

// LazyCoverCacheRepository delegates to postgres.CoverCacheRepository stored on PoolRef.
type LazyCoverCacheRepository struct {
	ref *PoolRef
}

// NewLazyCoverCacheRepository returns a postgres.CoverCacheRepository wrapping ref.
func NewLazyCoverCacheRepository(ref *PoolRef) postgres.CoverCacheRepository {
	return &LazyCoverCacheRepository{ref: ref}
}

var _ postgres.CoverCacheRepository = (*LazyCoverCacheRepository)(nil)

func (l *LazyCoverCacheRepository) get() (postgres.CoverCacheRepository, error) {
	repo, ok := l.ref.GetCoverCacheRepository()
	if !ok {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return repo, nil
}

func (l *LazyCoverCacheRepository) GetCover(ctx context.Context, coverID int64) (string, string, bool, bool, error) {
	repo, err := l.get()
	if err != nil {
		return "", "", false, false, err
	}
	return repo.GetCover(ctx, coverID)
}

func (l *LazyCoverCacheRepository) SaveCover(ctx context.Context, coverID int64, contentType string, data []byte) (string, error) {
	repo, err := l.get()
	if err != nil {
		return "", err
	}
	return repo.SaveCover(ctx, coverID, contentType, data)
}

func (l *LazyCoverCacheRepository) MarkMissing(ctx context.Context, coverID int64) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.MarkMissing(ctx, coverID)
}

// LazySourceRecordRepository delegates to postgres.SourceRecordRepository stored on PoolRef.
type LazySourceRecordRepository struct {
	ref *PoolRef
}

// NewLazySourceRecordRepository returns a SourceRecordRepository wrapping ref.
func NewLazySourceRecordRepository(ref *PoolRef) SourceRecordRepository {
	return &LazySourceRecordRepository{ref: ref}
}

var _ SourceRecordRepository = (*LazySourceRecordRepository)(nil)

func (l *LazySourceRecordRepository) get() (*postgres.SourceRecordRepository, error) {
	repo, ok := l.ref.GetSourceRecordRepository()
	if !ok {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "database not ready"}
	}
	return repo, nil
}

func (l *LazySourceRecordRepository) Create(ctx context.Context, rec postgres.SourceRecord) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.Create(ctx, rec)
}

func (l *LazySourceRecordRepository) Get(ctx context.Context, id string) (postgres.SourceRecord, error) {
	repo, err := l.get()
	if err != nil {
		return postgres.SourceRecord{}, err
	}
	return repo.Get(ctx, id)
}

func (l *LazySourceRecordRepository) List(ctx context.Context) ([]postgres.SourceRecord, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.List(ctx)
}

func (l *LazySourceRecordRepository) UpdateConfig(ctx context.Context, id, label, basePath, baseURL string) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.UpdateConfig(ctx, id, label, basePath, baseURL)
}

func (l *LazySourceRecordRepository) SetCredential(ctx context.Context, id string, ciphertext, nonce []byte) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.SetCredential(ctx, id, ciphertext, nonce)
}

func (l *LazySourceRecordRepository) UpdateHealth(ctx context.Context, id, status, detail string, checkedAt time.Time, caps domain.SourceCapabilities, searchLinkURL string) error {
	repo, err := l.get()
	if err != nil {
		return err
	}
	return repo.UpdateHealth(ctx, id, status, detail, checkedAt, caps, searchLinkURL)
}
