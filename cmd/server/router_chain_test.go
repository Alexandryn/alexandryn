package main

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"golang.org/x/time/rate"
)

// Middleware chain assembly: security-headers, CORS, and public
// rate-limit middlewares are mounted in newProductionRouter, in order,
// before auth.

func chainTestRouter(t *testing.T, origins []string) http.Handler {
	t.Helper()
	cfg := &config.Config{
		HTTPMaxBodyBytes:   1 << 20,
		CORSAllowedOrigins: origins,
	}
	limiter := auth.NewIPRateLimiter(rate.Every(time.Second/2), 60, time.Minute)
	return newProductionRouter(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), &transporthttp.PoolRef{}, limiter)
}

func TestChain_SecurityHeadersOnEveryResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	chainTestRouter(t, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if csp := rec.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("no Content-Security-Policy on /healthz")
	}
	// The served SPA HTML document, not only /api and /healthz, must carry
	// the CSP — it is the framing/injection target.
	spaRec := httptest.NewRecorder()
	chainTestRouter(t, nil).ServeHTTP(spaRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.HasPrefix(spaRec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("GET / content-type = %q, want text/html", spaRec.Header().Get("Content-Type"))
	}
	spaCSP := spaRec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(spaCSP, want) {
			t.Errorf("SPA CSP %q missing %q", spaCSP, want)
		}
	}
	if spaRec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("SPA X-Frame-Options = %q, want DENY", spaRec.Header().Get("X-Frame-Options"))
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

// Regression: HSTS was gated on cfg.TLSCertificate() != nil, which stays
// nil in ACME mode (only the static-cert path populates it) — a public
// ACME-terminated bind, the deployment where HSTS matters most, shipped
// without it. The gate is now an explicit TLSMode allow-list.
func TestChain_HSTSSetForACMEMode(t *testing.T) {
	t.Setenv("OPEN_LIBRARY_USER_AGENT", "Alexandryn/test")
	t.Setenv("BIND_ADDRESS", "library.example.com:443")
	t.Setenv("ACME_ENABLED", "true")
	t.Setenv("ACME_DOMAIN", "library.example.com")
	cfg, err := config.Load("", func(string) ([]byte, error) { return nil, fs.ErrNotExist }, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.TLSMode() != "acme" {
		t.Fatalf("TLSMode() = %q, want acme", cfg.TLSMode())
	}
	if cfg.TLSCertificate() != nil {
		t.Fatal("ACME mode must not populate a static certificate — this is exactly the case the old TLSCertificate()!=nil gate got wrong")
	}

	limiter := auth.NewIPRateLimiter(rate.Every(time.Second/2), 60, time.Minute)
	router := newProductionRouter(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), &transporthttp.PoolRef{}, limiter)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Fatalf("HSTS on an ACME-mode bind = %q, want max-age=31536000", got)
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
	// burst is 100 in newProductionRouter; well past it must 429.
	for i := 0; i < 150; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = "203.0.113.44:5000"
		router.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("150th rapid /healthz got %d, want 429 — the public rate limiter is not mounted", last)
	}
}

func TestChain_RateLimitRunsBeforeCORSPreflight(t *testing.T) {
	// A CORS-preflight-shaped flood on a health probe must be metered —
	// the rate limiter sits ahead of CORS, so CORS can't short-circuit it
	// with a 204 before the bucket is consulted.
	router := chainTestRouter(t, []string{"https://proxy.example"})
	var last int
	for i := 0; i < 150; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
		req.RemoteAddr = "203.0.113.77:6000"
		req.Header.Set("Origin", "https://proxy.example")
		req.Header.Set("Access-Control-Request-Method", "GET")
		router.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("preflight flood on /healthz ended at %d, want 429 — the limiter is behind CORS's 204 short-circuit", last)
	}
}

// Opening the bind must not make routes public unintentionally.
// Tests the set of paths marked public without authentication.
func TestIsPublicPathContract(t *testing.T) {
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

// TestRouter_OriginValidationOnSetupAndLogin reproduces the gap where
// OriginValidation was applied only to POST /network/pair/verify on the
// stated premise that every other state-changing route is
// Authorization-gated and therefore CSRF-safe by construction — but
// /api/v1/auth/setup and /api/v1/auth/login are also unauthenticated,
// state-changing POST routes (IsPublicPath), so that premise doesn't hold
// for them. A hostile page could otherwise drive a victim's browser to
// POST to either during the pre-first-run window.
func TestRouter_OriginValidationOnSetupAndLogin(t *testing.T) {
	router := chainTestRouter(t, []string{"https://allowed.example"})

	for _, path := range []string{"/api/v1/auth/setup", "/api/v1/auth/login"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://hostile.example")
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("%s with a foreign Origin: status = %d, want 403 — OriginValidation is not mounted", path, rec.Code)
		}
	}
}

func TestRouter_BootstrapRouteAccessible(t *testing.T) {
	router := chainTestRouter(t, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/bootstrap, got %d", rec.Code)
	}
}

func TestRouter_SourcesAndImportProtectedFromUnauthenticated(t *testing.T) {
	router := chainTestRouter(t, nil)

	// An unauthenticated request to /api/v1/sources must be rejected (either 401 or 503 if DB unready, but NEVER 200)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Errorf("unauthenticated /api/v1/sources succeeded with 200, want rejection")
	}

	// An unauthenticated request to /api/v1/import/discover must be rejected
	recImport := httptest.NewRecorder()
	reqImport := httptest.NewRequest(http.MethodPost, "/api/v1/import/discover", nil)
	router.ServeHTTP(recImport, reqImport)

	if recImport.Code == http.StatusOK {
		t.Errorf("unauthenticated /api/v1/import/discover succeeded with 200, want rejection")
	}
}
