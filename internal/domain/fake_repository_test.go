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

// fakeEditionRepository is the same pattern, for domain.EditionRepository.
type fakeEditionRepository struct {
	mu       sync.Mutex
	editions map[domain.EditionID]*domain.Edition
}

func newFakeEditionRepository(editions ...*domain.Edition) *fakeEditionRepository {
	r := &fakeEditionRepository{editions: map[domain.EditionID]*domain.Edition{}}
	for _, e := range editions {
		r.editions[e.ID()] = e
	}
	return r
}

func (r *fakeEditionRepository) FindByID(_ context.Context, id domain.EditionID) (*domain.Edition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.editions[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "edition not found"}
	}
	return e, nil
}

func (r *fakeEditionRepository) FindByWork(_ context.Context, workID domain.WorkID) ([]*domain.Edition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*domain.Edition
	for _, e := range r.editions {
		if e.WorkID() == workID {
			result = append(result, e)
		}
	}
	return result, nil
}

var _ domain.EditionRepository = (*fakeEditionRepository)(nil)

// fakeLibraryEntryRepository is the same pattern, for
// domain.LibraryEntryRepository, keyed by EditionID since that's the
// uniqueness key FR-7 requires.
type fakeLibraryEntryRepository struct {
	mu      sync.Mutex
	entries map[domain.EditionID]*domain.LibraryEntry
}

func newFakeLibraryEntryRepository(entries ...*domain.LibraryEntry) *fakeLibraryEntryRepository {
	r := &fakeLibraryEntryRepository{entries: map[domain.EditionID]*domain.LibraryEntry{}}
	for _, e := range entries {
		r.entries[e.EditionID()] = e
	}
	return r
}

func (r *fakeLibraryEntryRepository) FindByEdition(_ context.Context, editionID domain.EditionID) (*domain.LibraryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[editionID]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "library entry not found"}
	}
	return e, nil
}

func (r *fakeLibraryEntryRepository) Save(_ context.Context, e *domain.LibraryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[e.EditionID()] = e
	return nil
}

func (r *fakeLibraryEntryRepository) DeleteByEdition(_ context.Context, editionID domain.EditionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, editionID)
	return nil
}

var _ domain.LibraryEntryRepository = (*fakeLibraryEntryRepository)(nil)

// fakeCollectionRepository is the same pattern, for
// domain.CollectionRepository.
type fakeCollectionRepository struct {
	mu          sync.Mutex
	collections map[domain.CollectionID]*domain.Collection
}

func newFakeCollectionRepository(collections ...*domain.Collection) *fakeCollectionRepository {
	r := &fakeCollectionRepository{collections: map[domain.CollectionID]*domain.Collection{}}
	for _, c := range collections {
		r.collections[c.ID()] = c
	}
	return r
}

func (r *fakeCollectionRepository) FindByID(_ context.Context, id domain.CollectionID) (*domain.Collection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.collections[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}
	return c, nil
}

func (r *fakeCollectionRepository) Save(_ context.Context, c *domain.Collection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collections[c.ID()] = c
	return nil
}

func (r *fakeCollectionRepository) Delete(_ context.Context, id domain.CollectionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.collections, id)
	return nil
}

var _ domain.CollectionRepository = (*fakeCollectionRepository)(nil)
