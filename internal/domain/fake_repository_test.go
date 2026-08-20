package domain_test

import (
	"context"
	"sync"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// fakeWorkRepository is an in-memory domain.WorkRepository, used only by
// this package's own tests (never internal/testutil — this fake is
// domain-specific, not a cross-package fixture like Clock/FS).
type fakeWorkRepository struct {
	mu    sync.Mutex
	works map[domain.WorkID]*domain.Work
}

func newFakeWorkRepository(works ...*domain.Work) *fakeWorkRepository {
	r := &fakeWorkRepository{works: map[domain.WorkID]*domain.Work{}}
	for _, w := range works {
		r.works[w.ID()] = w
	}
	return r
}

func (r *fakeWorkRepository) FindByID(_ context.Context, id domain.WorkID) (*domain.Work, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.works[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "work not found"}
	}
	return w, nil
}

func (r *fakeWorkRepository) FindMergedInto(_ context.Context, canonical domain.WorkID) ([]*domain.Work, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*domain.Work
	for _, w := range r.works {
		if w.MergedInto() != nil && *w.MergedInto() == canonical {
			result = append(result, w)
		}
	}
	return result, nil
}

func (r *fakeWorkRepository) Save(_ context.Context, w *domain.Work) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.works[w.ID()] = w
	return nil
}

var _ domain.WorkRepository = (*fakeWorkRepository)(nil)

// fakeAuthorRepository is the same pattern, for domain.AuthorRepository.
type fakeAuthorRepository struct {
	mu      sync.Mutex
	authors map[domain.AuthorID]*domain.Author
}

func newFakeAuthorRepository(authors ...*domain.Author) *fakeAuthorRepository {
	r := &fakeAuthorRepository{authors: map[domain.AuthorID]*domain.Author{}}
	for _, a := range authors {
		r.authors[a.ID()] = a
	}
	return r
}

func (r *fakeAuthorRepository) FindByID(_ context.Context, id domain.AuthorID) (*domain.Author, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.authors[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "author not found"}
	}
	return a, nil
}

func (r *fakeAuthorRepository) Save(_ context.Context, a *domain.Author) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authors[a.ID()] = a
	return nil
}

var _ domain.AuthorRepository = (*fakeAuthorRepository)(nil)
