package auth

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu    sync.Mutex
	ips   map[string]*ipEntry
	rate  rate.Limit
	burst int
	ttl   time.Duration
}

func NewIPRateLimiter(r rate.Limit, burst int, ttl time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		ips:   make(map[string]*ipEntry),
		rate:  r,
		burst: burst,
		ttl:   ttl,
	}
}

func (i *IPRateLimiter) Allow(ip string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	now := time.Now()
	entry, exists := i.ips[ip]
	if !exists {
		limiter := rate.NewLimiter(i.rate, i.burst)
		i.ips[ip] = &ipEntry{
			limiter:  limiter,
			lastSeen: now,
		}
		return limiter.Allow()
	}

	entry.lastSeen = now
	return entry.limiter.Allow()
}

func (i *IPRateLimiter) Cleanup(now time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()

	for ip, entry := range i.ips {
		if now.Sub(entry.lastSeen) > i.ttl {
			delete(i.ips, ip)
		}
	}
}

// StartEviction runs Cleanup on a ticker until ctx is done. Without it the
// per-IP map only ever grows — an unauthenticated caller rotating source
// addresses (trivial over IPv6) would push it to OOM, turning the limiter
// that exists to shed a flood into a memory-exhaustion vector. Call it
// once per limiter, at startup, with the process context.
func (i *IPRateLimiter) StartEviction(ctx context.Context) {
	interval := i.ttl / 2
	if interval < time.Minute {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				i.Cleanup(now)
			}
		}
	}()
}
