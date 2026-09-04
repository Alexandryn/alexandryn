package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func corsReq(origin, method string) *http.Request {
	r := httptest.NewRequest(method, "/api/v1/library", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if method == http.MethodOptions {
		r.Header.Set("Access-Control-Request-Method", "GET")
	}
	return r
}

func TestCORS_EmptyAllowlistEmitsNothing(t *testing.T) {
	mw := transporthttp.CORS(nil)

	rec := serve(mw, corsReq("https://evil.example", http.MethodGet))
	for _, k := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Vary"} {
		if rec.Header().Get(k) != "" {
			t.Errorf("empty allowlist emitted %s", k)
		}
	}

	pre := serve(mw, corsReq("https://evil.example", http.MethodOptions))
	if pre.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", pre.Code)
	}
	if pre.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("preflight leaked Access-Control-Allow-Origin with an empty allowlist")
	}
}

func TestCORS_ExactMatchOnly(t *testing.T) {
	mw := transporthttp.CORS([]string{"https://proxy.example"})

	ok := serve(mw, corsReq("https://proxy.example", http.MethodGet))
	if ok.Header().Get("Access-Control-Allow-Origin") != "https://proxy.example" {
		t.Errorf("exact match not echoed: %q", ok.Header().Get("Access-Control-Allow-Origin"))
	}
	if ok.Header().Get("Vary") != "Origin" {
		t.Errorf("Vary = %q, want Origin", ok.Header().Get("Vary"))
	}
	if ok.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Error("Allow-Credentials must never be set")
	}

	for _, near := range []string{
		"http://proxy.example",       // scheme
		"https://proxy.example:443",  // explicit port
		"https://proxy.example/",     // trailing slash
		"https://sub.proxy.example",  // subdomain
		"https://proxy.example.evil", // suffix
	} {
		rec := serve(mw, corsReq(near, http.MethodGet))
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("near-miss %q was allowed", near)
		}
	}
}
