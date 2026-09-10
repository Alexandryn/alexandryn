package observability

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingPurger blocks the first PurgeExpired call until released, so a
// test can observe whether Reaper.Stop waits for an in-flight sweep.
// Later calls return immediately.
type blockingPurger struct {
	entered   chan struct{}
	enterOnce sync.Once
	release   chan struct{}
	finished  atomic.Bool
	calls     atomic.Int32
}

func (p *blockingPurger) PurgeExpired(_ context.Context, _ time.Time) (int64, error) {
	if p.calls.Add(1) > 1 {
		return 0, nil
	}
	p.enterOnce.Do(func() { close(p.entered) })
	<-p.release
	p.finished.Store(true)
	return 0, nil
}

// #303: Stop must not return until a sweep that is already running has
// finished — otherwise the pool can close under an in-flight query.
func TestReaper_StopWaitsForInFlightSweep(t *testing.T) {
	p := &blockingPurger{entered: make(chan struct{}), release: make(chan struct{})}
	r := NewReaper(p, 5*time.Millisecond, nil)
	r.Start(context.Background())

	select {
	case <-p.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("sweep never started")
	}

	stopReturned := make(chan struct{})
	go func() {
		r.Stop()
		close(stopReturned)
	}()

	select {
	case <-stopReturned:
		t.Fatal("Stop returned while a sweep was still running")
	case <-time.After(50 * time.Millisecond):
	}

	close(p.release)

	select {
	case <-stopReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after the sweep finished")
	}
	if !p.finished.Load() {
		t.Fatal("Stop returned before the sweep completed")
	}
}

func TestReaper_StopIsSafeWithoutStartAndTwice(t *testing.T) {
	r := NewReaper(&blockingPurger{entered: make(chan struct{}), release: make(chan struct{})}, time.Hour, nil)
	done := make(chan struct{})
	go func() {
		r.Stop()
		r.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop deadlocked when Start was never called")
	}
}
