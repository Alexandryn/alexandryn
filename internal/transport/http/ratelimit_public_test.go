package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"golang.org/x/time/rate"
)

func TestPublicRateLimit_BurstThen429_KeyedOnRemoteAddr(t *testing.T) {
	limiter := auth.NewIPRateLimiter(rate.Every(time.Minute), 2, time.Minute) // burst 2
	mw := transporthttp.PublicRateLimit(limiter, transporthttp.HealthProbePath)

	req := func(xff string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		r.RemoteAddr = "203.0.113.9:5555"
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return serve(mw, r)
	}

	for i := 1; i <= 2; i++ {
		if got := req("").Code; got != http.StatusOK {
			t.Fatalf("burst request %d status = %d, want 200", i, got)
		}
	}
	blocked := req("")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want 429", blocked.Code)
	}
	var body struct{ Code string }
	_ = json.NewDecoder(blocked.Body).Decode(&body)
	if body.Code != "RateLimited" {
		t.Errorf("429 body code = %q, want RateLimited", body.Code)
	}

	// A different X-Forwarded-For must NOT get a fresh bucket — the
	// limiter keys on RemoteAddr only.
	if spoof := req("198.51.100.1"); spoof.Code != http.StatusTooManyRequests {
		t.Errorf("X-Forwarded-For spoof got status %d — the limiter must ignore the header", spoof.Code)
	}
}

// #195: behind a configured trusted proxy the limiter must key on the
// last X-Forwarded-For hop, so two real clients arriving through one
// proxy get separate buckets; from an untrusted RemoteAddr the header is
// still ignored. IPv6 clients collapse to their /64.
func TestPublicRateLimit_TrustedProxyAndIPv6(t *testing.T) {
	mustPrefix := func(s string) netip.Prefix { return netip.MustParsePrefix(s) }
	transporthttp.SetTrustedProxyCIDRs([]netip.Prefix{mustPrefix("10.0.0.0/8")})
	t.Cleanup(func() { transporthttp.SetTrustedProxyCIDRs(nil) })

	t.Run("distinct XFF hops via a trusted proxy get distinct buckets", func(t *testing.T) {
		limiter := auth.NewIPRateLimiter(rate.Every(time.Minute), 1, time.Minute)
		mw := transporthttp.PublicRateLimit(limiter, transporthttp.HealthProbePath)
		call := func(xff string) int {
			r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			r.RemoteAddr = "10.1.2.3:9000" // the proxy, trusted
			r.Header.Set("X-Forwarded-For", xff)
			return serve(mw, r).Code
		}
		if got := call("203.0.113.1"); got != http.StatusOK {
			t.Fatalf("first client status = %d, want 200", got)
		}
		if got := call("203.0.113.1"); got != http.StatusTooManyRequests {
			t.Fatalf("first client second hit = %d, want 429", got)
		}
		if got := call("203.0.113.2"); got != http.StatusOK {
			t.Fatalf("second distinct client = %d, want 200 (separate bucket)", got)
		}
	})

	t.Run("XFF from an untrusted RemoteAddr is ignored", func(t *testing.T) {
		limiter := auth.NewIPRateLimiter(rate.Every(time.Minute), 1, time.Minute)
		mw := transporthttp.PublicRateLimit(limiter, transporthttp.HealthProbePath)
		call := func(xff string) int {
			r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			r.RemoteAddr = "203.0.113.9:5555" // not in 10/8
			r.Header.Set("X-Forwarded-For", xff)
			return serve(mw, r).Code
		}
		if got := call("198.51.100.1"); got != http.StatusOK {
			t.Fatalf("first = %d, want 200", got)
		}
		if got := call("198.51.100.2"); got != http.StatusTooManyRequests {
			t.Fatalf("spoofed XFF got a fresh bucket (%d) — header must be ignored from an untrusted peer", got)
		}
	})

	t.Run("IPv6 clients in one /64 share a bucket", func(t *testing.T) {
		limiter := auth.NewIPRateLimiter(rate.Every(time.Minute), 1, time.Minute)
		mw := transporthttp.PublicRateLimit(limiter, transporthttp.HealthProbePath)
		call := func(remote string) int {
			r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			r.RemoteAddr = remote
			return serve(mw, r).Code
		}
		if got := call("[2001:db8:abcd:1234::1]:40000"); got != http.StatusOK {
			t.Fatalf("first = %d, want 200", got)
		}
		if got := call("[2001:db8:abcd:1234:ffff::9]:40001"); got != http.StatusTooManyRequests {
			t.Fatalf("same /64, different host = %d, want 429 (shared bucket)", got)
		}
	})
}

func TestPublicRateLimit_OnlyTouchesHealthProbes(t *testing.T) {
	limiter := auth.NewIPRateLimiter(rate.Every(time.Hour), 0, time.Minute) // 0 burst — always denies if applied
	mw := transporthttp.PublicRateLimit(limiter, transporthttp.HealthProbePath)

	for _, path := range []string{"/api/v1/library", "/", "/assets/app.js"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = "203.0.113.9:5555"
		if rec := serve(mw, r); rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d — the health-probe limiter must not apply to static assets or the API", path, rec.Code)
		}
	}
}

func TestHealthProbePath(t *testing.T) {
	for path, want := range map[string]bool{
		"/healthz":        true,
		"/readyz":         true,
		"/":               false,
		"/assets/app.js":  false,
		"/api/v1/library": false,
	} {
		if got := transporthttp.HealthProbePath(path); got != want {
			t.Errorf("HealthProbePath(%q) = %v, want %v", path, got, want)
		}
	}
}
