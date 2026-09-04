package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// The phase-13 middleware-chain assembly (backend-http-transport.md FR-1
// as amended for ADR 0028): the security-headers, CORS and public
// rate-limit middlewares are mounted in newProductionRouter, in order,
// before auth.

func chainTestRouter(t *testing.T, origins []string) http.Handler {
	t.Helper()
	cfg := &config.Config{
		HTTPMaxBodyBytes:   1 << 20,
		CORSAllowedOrigins: origins,
	}
	return newProductionRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), &transporthttp.PoolRef{})
}

func TestChain_SecurityHeadersOnEveryResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	chainTestRouter(t, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if csp := rec.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("no Content-Security-Policy on /healthz")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options = %q", rec.Header().Get("X-Frame-Options"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", rec.Header().Get("X-Content-Type-Options"))
	}
	// No cert in this cfg -> plaintext bind -> no HSTS.
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS set on a plaintext bind")
	}
}

func TestChain_CORSDenyByDefault(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set("Origin", "https://evil.example")
	chainTestRouter(t, nil).ServeHTTP(rec, r)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("cross-origin request got Access-Control-Allow-Origin with an empty allowlist")
	}
}

func TestChain_CORSEchoesAConfiguredOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set("Origin", "https://proxy.example")
	chainTestRouter(t, []string{"https://proxy.example"}).ServeHTTP(rec, r)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://proxy.example" {
		t.Errorf("configured origin not echoed: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestChain_PublicRateLimitOnHealthz(t *testing.T) {
	router := chainTestRouter(t, nil)
	var last int
	// burst is 30 in newProductionRouter.
	for i := 0; i < 40; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = "203.0.113.44:5000"
		router.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("40th rapid /healthz got %d, want 429 — the public rate limiter is not mounted", last)
	}
}

// FR-12 (backend-network-transport.md): opening the bind must not make any
// route public. This is the characterization test — the set is exactly
// what phase 12 established until Tier 4 adds POST /network/pair/verify.
func TestFR12_IsPublicPathUnchanged(t *testing.T) {
	public := []string{"/healthz", "/readyz", "/api/v1/auth/setup/status", "/api/v1/auth/setup", "/api/v1/auth/login"}
	notPublic := []string{
		"/api/v1/library", "/api/v1/reading/export", "/api/v1/network/status",
		"/api/v1/network/settings", "/api/v1/network/pair/initiate",
	}
	for _, p := range public {
		if !transporthttp.IsPublicPath(p) {
			t.Errorf("%s should be public", p)
		}
	}
	for _, p := range notPublic {
		if transporthttp.IsPublicPath(p) {
			t.Errorf("%s must NOT be public — opening the bind grants no route anonymous access", p)
		}
	}
}
