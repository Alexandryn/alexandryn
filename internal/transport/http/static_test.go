package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func fixtureWebDist() fstest.MapFS {
	return fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<html>the app shell</html>")},
		"assets/app.js":  &fstest.MapFile{Data: []byte("console.log('app')")},
		"assets/app.css": &fstest.MapFile{Data: []byte("body{}")},
	}
}

// FR-8: a request matching a real embedded file gets that file, with the
// Content-Type its extension implies (Go's own mime.TypeByExtension via
// http.FileServerFS, never a hardcoded map).
func TestStaticHandler_ServesARealFileWithTheCorrectContentType(t *testing.T) {
	cases := []struct {
		path      string
		wantBody  string
		wantCTHas string // substring, since charset params vary by Go version/platform
	}{
		{"/", "the app shell", "text/html"},
		{"/assets/app.js", "console.log", "javascript"},
		{"/assets/app.css", "body{}", "text/css"},
	}

	handler := transporthttp.StaticHandler(fixtureWebDist())

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want it to contain %q", rec.Body.String(), tc.wantBody)
			}
			ct := rec.Header().Get("Content-Type")
			if !strings.Contains(ct, tc.wantCTHas) {
				t.Fatalf("Content-Type = %q, want it to contain %q", ct, tc.wantCTHas)
			}
		})
	}
}

// FR-8: a request matching no real file falls back to index.html (200,
// the SPA-fallback pattern frontend-shell-and-routing.md's client-side
// routing needs) rather than a 404.
func TestStaticHandler_UnknownPathFallsBackToIndexHTML(t *testing.T) {
	handler := transporthttp.StaticHandler(fixtureWebDist())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/library/some-book", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (SPA fallback), not a 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "the app shell") {
		t.Fatalf("body = %q, want the fallback index.html content", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
}

// FR-7/FR-8 boundary: an unmatched /api/v1/... path still gets the JSON
// 404, never the SPA fallback — proven together, not assumed, since
// they're two different catch-alls that could easily be mis-registered
// to shadow each other.
func TestRoutingPrecedence_APINotFoundNeverFallsBackToSPA(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", transporthttp.NotFoundHandler())
	mux.Handle("/", transporthttp.StaticHandler(fixtureWebDist()))

	t.Run("unmatched API path gets the JSON 404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json — the API 404 must never fall back to the SPA", ct)
		}
	})

	t.Run("unmatched non-API path gets the SPA fallback", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/library/some-book", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (SPA fallback)", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("Content-Type = %q, want text/html", ct)
		}
	})
}

// The real, embedded placeholder (D2) is a valid target too — proves the
// production wiring, not just the fixture-backed handler.
func TestDefaultStaticHandler_ServesTheEmbeddedPlaceholder(t *testing.T) {
	handler := transporthttp.DefaultStaticHandler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Alexandryn") {
		t.Fatalf("body doesn't look like the placeholder index.html: %s", rec.Body.String())
	}
}
