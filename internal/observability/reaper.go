package observability

import (
	"context"
	"log/slog"
	"time"
)

// Reaper runs a periodic background task to purge expired system_events rows.
type Reaper struct {
	store    *EventStore
	interval time.Duration
	logger   *slog.Logger
	stopCh   chan struct{}
}

// NewReaper constructs a retention reaper. If interval <= 0, 1 hour is used.
func NewReaper(store *EventStore, interval time.Duration, logger *slog.Logger) *Reaper {
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

// Start launches the background ticker goroutine. It terminates when ctx is cancelled or Stop is called.
func (r *Reaper) Start(ctx context.Context) {
	go func() {
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

// Stop halts the reaper ticker.
func (r *Reaper) Stop() {
	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}
