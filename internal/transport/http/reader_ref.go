package http

import (
	"sync/atomic"

	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

// readerRefs holds the phase-11 reader singletons populated once
// persistence is ready.
type readerRefs struct {
	contentCache atomic.Pointer[content.Cache]
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
