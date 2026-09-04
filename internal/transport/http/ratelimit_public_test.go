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
	mw := transporthttp.PublicRateLimit(limiter, transporthttp.HealthAndStaticPath)

	req := func(xff string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		r.RemoteAddr = "203.0.113.9:5555"
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return serve(mw, r)
	}

	if req("").Code != http.StatusOK || req("").Code != http.StatusOK {
		t.Fatal("first two requests within the burst should pass")
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

func TestPublicRateLimit_DoesNotTouchAPIPaths(t *testing.T) {
	limiter := auth.NewIPRateLimiter(rate.Every(time.Hour), 0, time.Minute) // 0 burst — always denies if applied
	mw := transporthttp.PublicRateLimit(limiter, transporthttp.HealthAndStaticPath)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/library", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	if rec := serve(mw, r); rec.Code != http.StatusOK {
		t.Fatalf("/api/v1 path status = %d — the health/static limiter must not apply", rec.Code)
	}
}

func TestHealthAndStaticPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/healthz":          true,
		"/readyz":           true,
		"/":                 true,
		"/assets/app.js":    true,
		"/api/v1/library":   false,
		"/api/v1/network/x": false,
	} {
		if got := transporthttp.HealthAndStaticPath(path); got != want {
			t.Errorf("HealthAndStaticPath(%q) = %v, want %v", path, got, want)
		}
	}
}
