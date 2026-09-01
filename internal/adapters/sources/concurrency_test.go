package sources_test

import (
	"sync"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
)

func TestSemaphore_CapRejectsImmediately(t *testing.T) {
	sem := sources.NewSemaphore(50)

	for i := 0; i < 50; i++ {
		if !sem.TryAcquire() {
			t.Fatalf("acquire %d failed below the cap", i)
		}
	}
	if sem.TryAcquire() {
		t.Fatal("51st acquire succeeded — the cap is not enforced")
	}
	if got := sem.InFlight(); got != 50 {
		t.Fatalf("InFlight = %d, want 50", got)
	}

	sem.Release()
	if !sem.TryAcquire() {
		t.Fatal("acquire after Release failed — a slot was not returned")
	}
}

func TestSemaphore_ReleaseWithoutAcquireIsSafe(t *testing.T) {
	sem := sources.NewSemaphore(2)
	sem.Release() // no matching acquire
	sem.Release()
	if !sem.TryAcquire() || !sem.TryAcquire() {
		t.Fatal("spurious Release corrupted the slot count")
	}
	if sem.TryAcquire() {
		t.Fatal("spurious Release inflated the cap")
	}
}

func TestSemaphore_ConcurrentAcquireRespectsCap(t *testing.T) {
	sem := sources.NewSemaphore(10)
	var granted int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sem.TryAcquire() {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if granted != 10 {
		t.Fatalf("granted = %d, want exactly 10", granted)
	}
}

func TestNewSemaphore_NonPositiveUsesDefault(t *testing.T) {
	sem := sources.NewSemaphore(0)
	n := 0
	for sem.TryAcquire() {
		n++
		if n > sources.DefaultOutboundLimit {
			t.Fatalf("acquired more than the default cap %d", sources.DefaultOutboundLimit)
		}
	}
	if n != sources.DefaultOutboundLimit {
		t.Fatalf("default cap = %d, want %d", n, sources.DefaultOutboundLimit)
	}
}
