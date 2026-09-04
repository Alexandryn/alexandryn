package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func serve(mw transporthttp.Middleware, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, r)
	return rec
}

func TestSecurityHeaders_SetOnEveryResponse(t *testing.T) {
	rec := serve(transporthttp.SecurityHeaders(), httptest.NewRequest(http.MethodGet, "/anything", nil))
	h := rec.Header()

	csp := h.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'", "base-uri 'none'", "form-action 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") || strings.Contains(csp, "script-src 'unsafe-inline'") {
		t.Errorf("script-src must not allow unsafe-inline: %s", csp)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if got := h.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q", got)
	}
	if got := h.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	if h.Get("Permissions-Policy") == "" {
		t.Error("Permissions-Policy not set")
	}
}

func TestHSTS_OnlyOnInProcessTLS(t *testing.T) {
	tls := serve(transporthttp.HSTS(true), httptest.NewRequest(http.MethodGet, "/", nil))
	if got := tls.Header().Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Errorf("HSTS on a TLS bind = %q, want max-age=31536000", got)
	}
	plain := serve(transporthttp.HSTS(false), httptest.NewRequest(http.MethodGet, "/", nil))
	if got := plain.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS on a plaintext bind = %q, want empty", got)
	}
}
