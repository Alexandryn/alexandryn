package content

import (
	"archive/zip"
	"context"
	"sync"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Clock is the injected time source (backend-test-harness.md FR-5) — no
// direct time.Now / timer, so idle eviction is testable without a real
// ten-minute wait.
type Clock interface {
	Now() time.Time
}

// systemClock is production's Clock.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Loader materialises an Edition's EPUB and returns an open zip.Reader
// over it plus a cleanup that closes the reader and removes the temp
// file. It runs under a cache-owned context (not any HTTP request's), so
// the temp file lives until the cache evicts the entry — never until the
// request that happened to populate it completes (backend-reader-content.md
// FR-2).
type Loader func(ctx context.Context, editionID domain.EditionID) (*zip.Reader, func(), error)

const (
	defaultMaxOpen     = 5
	defaultIdleTTL     = 10 * time.Minute
	defaultLoadTimeout = 30 * time.Second
)

// Cache holds up to a handful of open EPUB archives (FR-3): keyed by
// editionID, LRU-evicted at capacity, idle-evicted after a TTL, and
// reference-counted so an in-flight request pins its entry against
// eviction until it completes. Constructed once and injected, never a
// package global.
type Cache struct {
	mu          sync.Mutex
	clock       Clock
	loader      Loader
	maxOpen     int
	idleTTL     time.Duration
	loadTimeout time.Duration

	entries  map[domain.EditionID]*cacheEntry
	inflight map[domain.EditionID]*loadCall
}

type cacheEntry struct {
	reader   *zip.Reader
	cleanup  func()
	lastUsed time.Time
	refs     int
	evict    bool // eviction deferred until refs hits zero
}

type loadCall struct {
	done    chan struct{}
	reader  *zip.Reader
	cleanup func()
	err     error
}

// NewCache builds a Cache with the production wall clock and default
// bounds (FR-3: 5 open editions, 10-minute idle TTL).
func NewCache(loader Loader) *Cache {
	return newCache(loader, systemClock{}, defaultMaxOpen, defaultIdleTTL, defaultLoadTimeout)
}

func newCache(loader Loader, clock Clock, maxOpen int, idleTTL, loadTimeout time.Duration) *Cache {
	return &Cache{
		clock:       clock,
		loader:      loader,
		maxOpen:     maxOpen,
		idleTTL:     idleTTL,
		loadTimeout: loadTimeout,
		entries:     make(map[domain.EditionID]*cacheEntry),
		inflight:    make(map[domain.EditionID]*loadCall),
	}
}

// Handle is a borrowed reference to a cached archive. Release exactly once.
type Handle struct {
	reader  *zip.Reader
	release func()
}

func (h *Handle) Reader() *zip.Reader { return h.reader }

func (h *Handle) Release() { h.release() }

// Acquire returns a Handle to editionID's open archive, populating the
// cache on a miss. ctx bounds only the caller's willingness to wait; the
// load itself runs under a cache-owned, timeout-bounded context so a
// client disconnecting mid-load does not delete a temp file another
// request is about to use.
func (c *Cache) Acquire(ctx context.Context, editionID domain.EditionID) (*Handle, error) {
	c.mu.Lock()
	c.sweepLocked()

	if e, ok := c.entries[editionID]; ok {
		e.refs++
		e.lastUsed = c.clock.Now()
		c.mu.Unlock()
		return c.handleFor(editionID, e), nil
	}

	if call, ok := c.inflight[editionID]; ok {
		c.mu.Unlock()
		select {
		case <-call.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if call.err != nil {
			return nil, call.err
		}
		return c.Acquire(ctx, editionID)
	}

	call := &loadCall{done: make(chan struct{})}
	c.inflight[editionID] = call
	c.mu.Unlock()

	loadCtx, cancel := context.WithTimeout(context.Background(), c.loadTimeout)
	reader, cleanup, err := c.loader(loadCtx, editionID)

	c.mu.Lock()
	delete(c.inflight, editionID)
	if err != nil {
		cancel()
		call.err = err
		close(call.done)
		c.mu.Unlock()
		return nil, err
	}

	wrappedCleanup := func() { cleanup(); cancel() }
	e := &cacheEntry{reader: reader, cleanup: wrappedCleanup, lastUsed: c.clock.Now(), refs: 1}
	c.entries[editionID] = e
	c.evictLRULocked()

	call.reader = reader
	call.cleanup = wrappedCleanup
	close(call.done)
	c.mu.Unlock()

	return c.handleFor(editionID, e), nil
}

func (c *Cache) handleFor(editionID domain.EditionID, e *cacheEntry) *Handle {
	var once sync.Once
	return &Handle{
		reader: e.reader,
		release: func() {
			once.Do(func() {
				c.mu.Lock()
				defer c.mu.Unlock()
				e.refs--
				e.lastUsed = c.clock.Now()
				if e.refs == 0 && e.evict {
					delete(c.entries, editionID)
					e.cleanup()
				}
			})
		},
	}
}

// sweepLocked idle-evicts entries untouched for longer than idleTTL and
// not currently referenced. Called on every Acquire, so tests drive it
// by advancing the injected Clock.
func (c *Cache) sweepLocked() {
	now := c.clock.Now()
	for id, e := range c.entries {
		if e.refs == 0 && now.Sub(e.lastUsed) >= c.idleTTL {
			delete(c.entries, id)
			e.cleanup()
		}
	}
}

// evictLRULocked brings the cache back to maxOpen after an insert by
// closing the least-recently-used unreferenced entry. A referenced entry
// is marked for deferred eviction instead (closed when its last handle
// is released) — never yanked from under an in-flight read.
func (c *Cache) evictLRULocked() {
	for len(c.entries) > c.maxOpen {
		// Prefer the LRU unreferenced entry — evict it now.
		var freeID domain.EditionID
		var free *cacheEntry
		for id, e := range c.entries {
			if e.evict || e.refs != 0 {
				continue
			}
			if free == nil || e.lastUsed.Before(free.lastUsed) {
				freeID, free = id, e
			}
		}
		if free != nil {
			delete(c.entries, freeID)
			free.cleanup()
			continue
		}
		// Everything over capacity is referenced: mark the LRU one for
		// deferred eviction (closed on its last Release) and stop —
		// tolerate temporary over-capacity rather than yank a live read.
		var lru *cacheEntry
		for _, e := range c.entries {
			if e.evict {
				continue
			}
			if lru == nil || e.lastUsed.Before(lru.lastUsed) {
				lru = e
			}
		}
		if lru != nil {
			lru.evict = true
		}
		return
	}
}

// Sweep runs an idle-eviction pass on demand (production may call it from
// a periodic task; tests call it directly).
func (c *Cache) Sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked()
}

// Close evicts everything — for server shutdown.
func (c *Cache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, e := range c.entries {
		delete(c.entries, id)
		e.cleanup()
	}
}

// len reports the current entry count (test helper).
func (c *Cache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
