package http

import (
	"context"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
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

func (l *LazyWorkRepository) FindWorkDetail(ctx context.Context, id domain.WorkID) (*domain.WorkDetail, error) {
	repo, err := l.get()
	if err != nil {
		return nil, err
	}
	return repo.FindWorkDetail(ctx, id)
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

