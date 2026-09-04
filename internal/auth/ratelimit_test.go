package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"golang.org/x/time/rate"
)

func TestIPRateLimiter(t *testing.T) {
	// Limit 2 requests per second with burst 2
	limiter := auth.NewIPRateLimiter(rate.Every(500*time.Millisecond), 2, 5*time.Minute)

	ip1 := "192.168.1.100"
	ip2 := "10.0.0.1"

	// IP 1 first 2 requests succeed
	if !limiter.Allow(ip1) {
		t.Error("expected first request for ip1 to be allowed")
	}
	if !limiter.Allow(ip1) {
		t.Error("expected second request for ip1 to be allowed")
	}
	// 3rd immediate request exceeds burst
	if limiter.Allow(ip1) {
		t.Error("expected 3rd immediate request for ip1 to be blocked")
	}

	// IP 2 is independent
	if !limiter.Allow(ip2) {
		t.Error("expected first request for ip2 to be allowed")
	}
}

func TestIPRateLimiter_StartEviction(t *testing.T) {
	// ttl/2 with a tiny ttl -> the interval is clamped up to 1 minute, so
	// exercise Cleanup directly plus prove StartEviction stops on ctx.
	limiter := auth.NewIPRateLimiter(rate.Every(time.Second), 1, 10*time.Millisecond)
	limiter.Allow("203.0.113.1")

	ctx, cancel := context.WithCancel(context.Background())
	limiter.StartEviction(ctx)
	cancel() // must return without leaking the goroutine

	time.Sleep(20 * time.Millisecond)
	limiter.Cleanup(time.Now()) // the ip is now older than the 10ms ttl
	if !limiter.Allow("203.0.113.1") {
		t.Fatal("after Cleanup past the TTL the entry should be gone and a fresh bucket allows")
	}
}
