package http

import (
	"context"
	"sync/atomic"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

// ReadingExportStore is the bulk-read surface the export handler needs
// (reading-data-export.md) — satisfied by
// *postgres.ReadingExportRepository, kept as an interface so no postgres
// row shape reaches the handler and the handler is unit-testable.
type ReadingExportStore interface {
	WorkExists(ctx context.Context, workID string) (bool, error)
	ListProgress(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, workID string) ([]postgres.ExportProgress, error)
	ListMarks(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, workID string) ([]postgres.ExportMark, error)
}

// readerRefs holds the phase-11 reader singletons populated once
// persistence is ready.
type readerRefs struct {
	contentCache atomic.Pointer[content.Cache]
	readingAPI   atomic.Pointer[ReadingAPI]
}

// ReadingAPI bundles every repository the /api/v1/reading* handlers need
// (backend-reading-api.md, reading-data-export.md) — set once when
// persistence is ready, never a package global.
type ReadingAPI struct {
	Progress       domain.ReadingProgressRepository
	Bookmarks      domain.BookmarkRepository
	Highlights     domain.HighlightRepository
	Preferences    domain.ReadingPreferencesRepository
	Editions       domain.EditionRepository
	LibraryEntries domain.LibraryEntryRepository
	Devices        domain.PairedDeviceRepository
	Transactor     domain.Transactor
	IDs            domain.IDGenerator
	Export         ReadingExportStore
}

// SetReaderContentCache stores the content cache once its resolver's
// dependencies are all available (backend-reader-content.md FR-3 — built
// once and injected, never a package global).
func (r *PoolRef) SetReaderContentCache(c *content.Cache) {
	r.reader.contentCache.Store(c)
}

// GetReaderContentCache returns the content cache and whether one has
// been set.
func (r *PoolRef) GetReaderContentCache() (*content.Cache, bool) {
	c := r.reader.contentCache.Load()
	if c == nil {
		return nil, false
	}
	return c, true
}

// SetReadingAPI stores the reading-API repository bundle.
func (r *PoolRef) SetReadingAPI(a ReadingAPI) {
	r.reader.readingAPI.Store(&a)
}

// GetReadingAPI returns the reading-API bundle and whether it is ready.
func (r *PoolRef) GetReadingAPI() (ReadingAPI, bool) {
	a := r.reader.readingAPI.Load()
	if a == nil {
		return ReadingAPI{}, false
	}
	return *a, true
}
