package http

import (
	"context"

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
