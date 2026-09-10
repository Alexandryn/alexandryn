package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// expiredPurger is the one operation the reaper needs from the event
// store — an interface so a test can substitute a slow implementation and
// assert Stop waits for an in-flight sweep. *EventStore satisfies it.
type expiredPurger interface {
	PurgeExpired(ctx context.Context, now time.Time) (int64, error)
}

// Reaper runs a periodic background task to purge expired system_events rows.
type Reaper struct {
	store    expiredPurger
	interval time.Duration
	logger   *slog.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewReaper constructs a retention reaper. If interval <= 0, 1 hour is used.
func NewReaper(store expiredPurger, interval time.Duration, logger *slog.Logger) *Reaper {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Reaper{
		store:    store,
		interval: interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// RunSweep executes one retention purge pass.
func (r *Reaper) RunSweep(ctx context.Context, now time.Time) (int64, error) {
	purged, err := r.store.PurgeExpired(ctx, now)
	if err != nil {
		r.logger.Warn("retention reaper sweep failed",
			"error", err,
		)
		return 0, err
	}
	if purged > 0 {
		r.logger.Debug("purged expired system events",
			"count", purged,
		)
	}
	return purged, nil
}

// Start launches the background ticker goroutine. It terminates when ctx
// is cancelled or Stop is called. A sweep already running when the stop
// signal arrives runs to completion before the goroutine exits, so a
// caller that waits on Stop knows no sweep is still touching the store.
func (r *Reaper) Start(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stopCh:
				return
			case now := <-ticker.C:
				_, _ = r.RunSweep(ctx, now)
			}
		}
	}()
}

// Stop signals the reaper to halt and blocks until its goroutine has
// exited, including any sweep in flight. Safe to call more than once and
// safe to call when Start was never called.
func (r *Reaper) Stop() {
	r.stopOnce.Do(func() { close(r.stopCh) })
	r.wg.Wait()
}
