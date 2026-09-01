package content

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// fakeClock is a settable clock local to this test (testutil is a
// separate module path; content's own tiny fake avoids the import).
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type loaderStub struct {
	calls   int64
	cleaned map[domain.EditionID]*int64
	mu      sync.Mutex
	block   chan struct{} // if non-nil, load blocks until closed
}

func (l *loaderStub) load(ctx context.Context, id domain.EditionID) (*zip.Reader, func(), error) {
	atomic.AddInt64(&l.calls, 1)
	if l.block != nil {
		select {
		case <-l.block:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	l.mu.Lock()
	if l.cleaned == nil {
		l.cleaned = map[domain.EditionID]*int64{}
	}
	if l.cleaned[id] == nil {
		l.cleaned[id] = new(int64)
	}
	counter := l.cleaned[id]
	l.mu.Unlock()
	zr := tinyZipForTest()
	return zr, func() { atomic.AddInt64(counter, 1) }, nil
}

func (l *loaderStub) cleanupCount(id domain.EditionID) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cleaned[id] == nil {
		return 0
	}
	return atomic.LoadInt64(l.cleaned[id])
}

func tinyZipForTest() *zip.Reader {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("a.txt")
	_, _ = w.Write([]byte("hi"))
	_ = zw.Close()
	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	return zr
}

// FR-3: a second request for an already-cached Edition does not re-load.
func TestCache_HitDoesNotReload(t *testing.T) {
	l := &loaderStub{}
	c := newCache(l.load, &fakeClock{now: time.Unix(0, 0)}, 5, 10*time.Minute, time.Second)

	h1, err := c.Acquire(context.Background(), "edition-1")
	if err != nil {
		t.Fatal(err)
	}
	h1.Release()
	h2, err := c.Acquire(context.Background(), "edition-1")
	if err != nil {
		t.Fatal(err)
	}
	h2.Release()

	if got := atomic.LoadInt64(&l.calls); got != 1 {
		t.Fatalf("loader called %d times, want 1 (second request is a cache hit)", got)
	}
}

// FR-3: idle eviction after the TTL, driven by the injected Clock,
// closes the reader and removes the temp file.
func TestCache_IdleEvictionUsesInjectedClock(t *testing.T) {
	l := &loaderStub{}
	clk := &fakeClock{now: time.Unix(1000, 0)}
	c := newCache(l.load, clk, 5, 10*time.Minute, time.Second)

	h, _ := c.Acquire(context.Background(), "edition-1")
	h.Release()

	clk.advance(9 * time.Minute)
	c.Sweep()
	if c.len() != 1 {
		t.Fatalf("evicted before the TTL: len = %d", c.len())
	}

	clk.advance(2 * time.Minute)
	c.Sweep()
	if c.len() != 0 {
		t.Fatalf("not evicted after the TTL: len = %d", c.len())
	}
	if l.cleanupCount("edition-1") != 1 {
		t.Fatalf("cleanup not called on eviction: %d", l.cleanupCount("edition-1"))
	}
}

// FR-3: LRU eviction at capacity.
func TestCache_LRUEvictionAtCapacity(t *testing.T) {
	l := &loaderStub{}
	clk := &fakeClock{now: time.Unix(0, 0)}
	c := newCache(l.load, clk, 2, time.Hour, time.Second)

	for i := 1; i <= 3; i++ {
		clk.advance(time.Second)
		h, err := c.Acquire(context.Background(), domain.EditionID(fmt.Sprintf("edition-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		h.Release()
	}

	if c.len() != 2 {
		t.Fatalf("len = %d, want 2 (capacity)", c.len())
	}
	if l.cleanupCount("edition-1") != 1 {
		t.Fatalf("edition-1 (LRU) was not evicted")
	}
}

// FR-3: a referenced entry survives an eviction sweep that would
// otherwise remove it — closed only when its last handle is released.
func TestCache_ReferencedEntrySurvivesEviction(t *testing.T) {
	l := &loaderStub{}
	clk := &fakeClock{now: time.Unix(0, 0)}
	c := newCache(l.load, clk, 1, time.Hour, time.Second)

	pinned, _ := c.Acquire(context.Background(), "edition-1")

	// Force capacity pressure while edition-1 is still held.
	clk.advance(time.Second)
	h2, _ := c.Acquire(context.Background(), "edition-2")
	h2.Release()

	if l.cleanupCount("edition-1") != 0 {
		t.Fatalf("held entry was cleaned up while a handle was live")
	}
	// The reader is still usable through the live handle.
	if len(pinned.Reader().File) == 0 {
		t.Fatalf("pinned reader is empty")
	}

	pinned.Release()
	if l.cleanupCount("edition-1") != 1 {
		t.Fatalf("deferred eviction did not fire on final release: %d", l.cleanupCount("edition-1"))
	}
}

// FR-2: concurrent first requests for the same Edition load it once.
func TestCache_ConcurrentMissLoadsOnce(t *testing.T) {
	l := &loaderStub{block: make(chan struct{})}
	c := newCache(l.load, &fakeClock{now: time.Unix(0, 0)}, 5, time.Hour, time.Second)

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h, err := c.Acquire(context.Background(), "edition-1")
			errs[i] = err
			if err == nil {
				h.Release()
			}
		}(i)
	}
	time.Sleep(20 * time.Millisecond)
	close(l.block)
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("acquire failed: %v", err)
		}
	}
	if got := atomic.LoadInt64(&l.calls); got != 1 {
		t.Fatalf("loader called %d times under a concurrent miss, want 1", got)
	}
}
