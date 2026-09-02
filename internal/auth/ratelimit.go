package auth

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu      sync.Mutex
	ips     map[string]*ipEntry
	rate    rate.Limit
	burst   int
	ttl     time.Duration
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
