package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
