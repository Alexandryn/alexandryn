package openlibrary

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// RateLimiter bounds outbound Open Library requests per upstream usage policy.
type RateLimiter struct {
	limiter       *rate.Limiter
	waiterCap     int64
	activeWaiters int64
	logger        *slog.Logger
}

// NewRateLimiter creates a RateLimiter with the given rate (req/sec), burst, and max concurrent waiters.
func NewRateLimiter(r float64, burst int, waiterCap int64, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{
		limiter:   rate.NewLimiter(rate.Limit(r), burst),
		waiterCap: waiterCap,
		logger:    logger,
	}
}

// DefaultRateLimiter returns the standard Open Library limiter (3 req/sec, burst 3, 50 waiters).
func DefaultRateLimiter(logger *slog.Logger) *RateLimiter {
	return NewRateLimiter(3.0, 3, 50, logger)
}

// Wait blocks until a token is available or the 5-second wait budget expires.
func (r *RateLimiter) Wait(ctx context.Context) error {
	current := atomic.AddInt64(&r.activeWaiters, 1)
	defer atomic.AddInt64(&r.activeWaiters, -1)

	if current > r.waiterCap {
		if r.logger != nil {
			r.logger.Warn("Open Library rate limiter waiter cap exhausted",
				slog.Int64("active_waiters", current),
				slog.Int64("max_waiters", r.waiterCap),
			)
		}
		return &domain.Error{
			Category: domain.Unavailable,
			Message:  "Open Library rate limit queue is full, try again shortly",
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.limiter.Wait(waitCtx); err != nil {
		if r.logger != nil {
			r.logger.Warn("Open Library rate limiter wait budget expired", slog.Any("error", err))
		}
		return &domain.Error{
			Category: domain.Unavailable,
			Message:  "Open Library is busy, request timed out waiting for rate limit token",
		}
	}

	return nil
}
