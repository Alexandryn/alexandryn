package domain_test

import (
	"context"
	"sync"
	"time"

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

func (r *fakeWorkRepository) QueryLibrary(_ context.Context, _ domain.LibraryQuery) (*domain.LibraryPage, error) {
	return &domain.LibraryPage{Works: []*domain.WorkSummary{}}, nil
}

func (r *fakeWorkRepository) FindWorkDetail(_ context.Context, id domain.WorkID) (*domain.WorkDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.works[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "work not found"}
	}
	return &domain.WorkDetail{
		ID:       w.ID(),
		Title:    w.Title(),
		Subtitle: w.Subtitle(),
	}, nil
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

func (r *fakeEditionRepository) Save(_ context.Context, e *domain.Edition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.editions[e.ID()] = e
	return nil
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

func (r *fakeLibraryEntryRepository) EditionInLibrary(_ context.Context, editionID domain.EditionID, _ domain.LibraryID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.entries[editionID]
	return ok, nil
}

func (r *fakeLibraryEntryRepository) WorkInLibrary(_ context.Context, _ domain.WorkID, _ domain.LibraryID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries) > 0, nil
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

func (r *fakeCollectionRepository) FindAll(_ context.Context) ([]*domain.CollectionSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*domain.CollectionSummary
	for _, c := range r.collections {
		result = append(result, &domain.CollectionSummary{
			ID:        c.ID(),
			Name:      c.Name(),
			WorkCount: len(c.Members()),
		})
	}
	return result, nil
}

func (r *fakeCollectionRepository) FindDetail(_ context.Context, id domain.CollectionID) (*domain.CollectionDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.collections[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}
	return &domain.CollectionDetail{
		ID:    c.ID(),
		Name:  c.Name(),
		Works: []*domain.WorkSummary{},
	}, nil
}

func (r *fakeCollectionRepository) AddMember(_ context.Context, collectionID domain.CollectionID, workID domain.WorkID, addedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.collections[collectionID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}
	c.AddMember(workID, addedAt)
	return nil
}

func (r *fakeCollectionRepository) RemoveMember(_ context.Context, collectionID domain.CollectionID, workID domain.WorkID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.collections[collectionID]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}
	var found bool
	for _, m := range c.Members() {
		if m.WorkID == workID {
			found = true
			break
		}
	}
	if !found {
		return &domain.Error{Category: domain.NotFound, Message: "no membership found for that work in this collection"}
	}
	c.RemoveMember(workID)
	return nil
}

func (r *fakeCollectionRepository) Rename(_ context.Context, id domain.CollectionID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.collections[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
	}
	renamed, err := domain.NewCollection(c.ID(), name)
	if err != nil {
		return err
	}
	for _, m := range c.Members() {
		renamed.AddMember(m.WorkID, m.AddedAt)
	}
	r.collections[id] = renamed
	return nil
}

var _ domain.CollectionRepository = (*fakeCollectionRepository)(nil)

// fakeSourceRepository is the same pattern, for domain.SourceRepository.
type fakeSourceRepository struct {
	mu      sync.Mutex
	sources map[domain.SourceID]*domain.Source
}

func newFakeSourceRepository(sources ...*domain.Source) *fakeSourceRepository {
	r := &fakeSourceRepository{sources: map[domain.SourceID]*domain.Source{}}
	for _, s := range sources {
		r.sources[s.ID()] = s
	}
	return r
}

func (r *fakeSourceRepository) FindByID(_ context.Context, id domain.SourceID) (*domain.Source, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sources[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	return s, nil
}

func (r *fakeSourceRepository) Save(_ context.Context, s *domain.Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[s.ID()] = s
	return nil
}

func (r *fakeSourceRepository) Delete(_ context.Context, id domain.SourceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sources, id)
	return nil
}

var _ domain.SourceRepository = (*fakeSourceRepository)(nil)

// fakeSourceOfferingRepository is the same pattern, for
// domain.SourceOfferingRepository.
type fakeSourceOfferingRepository struct {
	mu        sync.Mutex
	offerings map[domain.SourceOfferingID]*domain.SourceOffering
}

func newFakeSourceOfferingRepository(offerings ...*domain.SourceOffering) *fakeSourceOfferingRepository {
	r := &fakeSourceOfferingRepository{offerings: map[domain.SourceOfferingID]*domain.SourceOffering{}}
	for _, o := range offerings {
		r.offerings[o.ID()] = o
	}
	return r
}

func (r *fakeSourceOfferingRepository) FindByID(_ context.Context, id domain.SourceOfferingID) (*domain.SourceOffering, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.offerings[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "source offering not found"}
	}
	return o, nil
}

func (r *fakeSourceOfferingRepository) FindBySource(_ context.Context, sourceID domain.SourceID) ([]*domain.SourceOffering, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*domain.SourceOffering
	for _, o := range r.offerings {
		if o.SourceID() == sourceID {
			result = append(result, o)
		}
	}
	return result, nil
}

func (r *fakeSourceOfferingRepository) FindByEdition(_ context.Context, editionID domain.EditionID) ([]*domain.SourceOffering, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*domain.SourceOffering
	for _, o := range r.offerings {
		if o.EditionID() == editionID {
			result = append(result, o)
		}
	}
	return result, nil
}

func (r *fakeSourceOfferingRepository) Save(_ context.Context, o *domain.SourceOffering) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offerings[o.ID()] = o
	return nil
}

func (r *fakeSourceOfferingRepository) Delete(_ context.Context, id domain.SourceOfferingID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.offerings, id)
	return nil
}

var _ domain.SourceOfferingRepository = (*fakeSourceOfferingRepository)(nil)

// fakeReadingProgressRepository is the same pattern, for
// domain.ReadingProgressRepository, keyed by WorkID since FR-1's
// singleton-per-Work invariant makes that the real uniqueness key.
type fakeReadingProgressRepository struct {
	mu       sync.Mutex
	progress map[domain.WorkID]*domain.ReadingProgress
}

func newFakeReadingProgressRepository(progress ...*domain.ReadingProgress) *fakeReadingProgressRepository {
	r := &fakeReadingProgressRepository{progress: map[domain.WorkID]*domain.ReadingProgress{}}
	for _, p := range progress {
		r.progress[p.WorkID()] = p
	}
	return r
}

func (r *fakeReadingProgressRepository) FindByWork(_ context.Context, workID domain.WorkID) (*domain.ReadingProgress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.progress[workID]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "reading progress not found"}
	}
	return p, nil
}

func (r *fakeReadingProgressRepository) FindByWorkForUpdate(ctx context.Context, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return r.FindByWork(ctx, workID)
}

func (r *fakeReadingProgressRepository) Save(_ context.Context, p *domain.ReadingProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress[p.WorkID()] = p
	return nil
}

func (r *fakeReadingProgressRepository) FindByWorkAndUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return r.FindByWork(ctx, workID)
}

func (r *fakeReadingProgressRepository) FindByWorkAndUserForUpdate(ctx context.Context, _ domain.UserID, _ domain.LibraryID, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return r.FindByWorkForUpdate(ctx, workID)
}

func (r *fakeReadingProgressRepository) SaveForUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, p *domain.ReadingProgress) error {
	return r.Save(ctx, p)
}

var _ domain.ReadingProgressRepository = (*fakeReadingProgressRepository)(nil)

// fakeBookmarkRepository is the same pattern, for domain.BookmarkRepository.
type fakeBookmarkRepository struct {
	mu        sync.Mutex
	bookmarks map[domain.BookmarkID]*domain.Bookmark
}

func newFakeBookmarkRepository(bookmarks ...*domain.Bookmark) *fakeBookmarkRepository {
	r := &fakeBookmarkRepository{bookmarks: map[domain.BookmarkID]*domain.Bookmark{}}
	for _, b := range bookmarks {
		r.bookmarks[b.ID()] = b
	}
	return r
}

func (r *fakeBookmarkRepository) FindByID(_ context.Context, id domain.BookmarkID) (*domain.Bookmark, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.bookmarks[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "bookmark not found"}
	}
	return b, nil
}

func (r *fakeBookmarkRepository) FindByEdition(_ context.Context, editionID domain.EditionID) ([]*domain.Bookmark, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*domain.Bookmark
	for _, b := range r.bookmarks {
		if b.EditionID() == editionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (r *fakeBookmarkRepository) FindByEditionAndUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, editionID domain.EditionID) ([]*domain.Bookmark, error) {
	return r.FindByEdition(ctx, editionID)
}

func (r *fakeBookmarkRepository) FindByIDAndUser(ctx context.Context, _ domain.UserID, id domain.BookmarkID) (*domain.Bookmark, error) {
	return r.FindByID(ctx, id)
}

func (r *fakeBookmarkRepository) DeleteAndUser(ctx context.Context, _ domain.UserID, id domain.BookmarkID) error {
	return r.Delete(ctx, id)
}

func (r *fakeBookmarkRepository) Save(_ context.Context, b *domain.Bookmark) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bookmarks[b.ID()] = b
	return nil
}

func (r *fakeBookmarkRepository) SaveForUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, b *domain.Bookmark) error {
	return r.Save(ctx, b)
}

func (r *fakeBookmarkRepository) Delete(_ context.Context, id domain.BookmarkID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bookmarks, id)
	return nil
}

var _ domain.BookmarkRepository = (*fakeBookmarkRepository)(nil)

// fakeHighlightRepository is the same pattern, for domain.HighlightRepository.
type fakeHighlightRepository struct {
	mu         sync.Mutex
	highlights map[domain.HighlightID]*domain.Highlight
}

func newFakeHighlightRepository(highlights ...*domain.Highlight) *fakeHighlightRepository {
	r := &fakeHighlightRepository{highlights: map[domain.HighlightID]*domain.Highlight{}}
	for _, h := range highlights {
		r.highlights[h.ID()] = h
	}
	return r
}

func (r *fakeHighlightRepository) FindByID(_ context.Context, id domain.HighlightID) (*domain.Highlight, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.highlights[id]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "highlight not found"}
	}
	return h, nil
}

func (r *fakeHighlightRepository) FindByEdition(_ context.Context, editionID domain.EditionID) ([]*domain.Highlight, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*domain.Highlight
	for _, h := range r.highlights {
		if h.EditionID() == editionID {
			result = append(result, h)
		}
	}
	return result, nil
}

func (r *fakeHighlightRepository) FindByEditionAndUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, editionID domain.EditionID) ([]*domain.Highlight, error) {
	return r.FindByEdition(ctx, editionID)
}

func (r *fakeHighlightRepository) FindByIDAndUser(ctx context.Context, _ domain.UserID, id domain.HighlightID) (*domain.Highlight, error) {
	return r.FindByID(ctx, id)
}

func (r *fakeHighlightRepository) DeleteAndUser(ctx context.Context, _ domain.UserID, id domain.HighlightID) error {
	return r.Delete(ctx, id)
}

func (r *fakeHighlightRepository) Save(_ context.Context, h *domain.Highlight) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.highlights[h.ID()] = h
	return nil
}

func (r *fakeHighlightRepository) SaveForUser(ctx context.Context, _ domain.UserID, _ domain.LibraryID, h *domain.Highlight) error {
	return r.Save(ctx, h)
}

func (r *fakeHighlightRepository) UpdateNoteCategoryAndUser(_ context.Context, _ domain.UserID, id domain.HighlightID, note, category string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.highlights[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound, Message: "highlight not found"}
	}
	r.highlights[id] = domain.NewHighlight(h.ID(), h.EditionID(), h.StartPosition(), h.EndPosition(), note, category, h.CreatedAt())
	return nil
}

func (r *fakeHighlightRepository) Delete(_ context.Context, id domain.HighlightID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.highlights, id)
	return nil
}

var _ domain.HighlightRepository = (*fakeHighlightRepository)(nil)

// fakeReadingPreferencesRepository is the same pattern, for
// domain.ReadingPreferencesRepository, keyed by DeviceID (FR-5).
type fakeReadingPreferencesRepository struct {
	mu    sync.Mutex
	prefs map[domain.DeviceID]*domain.ReadingPreferences
}

func newFakeReadingPreferencesRepository(prefs ...*domain.ReadingPreferences) *fakeReadingPreferencesRepository {
	r := &fakeReadingPreferencesRepository{prefs: map[domain.DeviceID]*domain.ReadingPreferences{}}
	for _, p := range prefs {
		r.prefs[p.DeviceID()] = p
	}
	return r
}

func (r *fakeReadingPreferencesRepository) FindByDevice(_ context.Context, deviceID domain.DeviceID) (*domain.ReadingPreferences, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.prefs[deviceID]
	if !ok {
		return nil, &domain.Error{Category: domain.NotFound, Message: "reading preferences not found"}
	}
	return p, nil
}

func (r *fakeReadingPreferencesRepository) FindByUserAndDevice(ctx context.Context, _ domain.UserID, deviceID domain.DeviceID) (*domain.ReadingPreferences, error) {
	return r.FindByDevice(ctx, deviceID)
}

func (r *fakeReadingPreferencesRepository) Save(_ context.Context, p *domain.ReadingPreferences) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefs[p.DeviceID()] = p
	return nil
}

func (r *fakeReadingPreferencesRepository) SaveForUser(ctx context.Context, _ domain.UserID, p *domain.ReadingPreferences) error {
	return r.Save(ctx, p)
}

var _ domain.ReadingPreferencesRepository = (*fakeReadingPreferencesRepository)(nil)
