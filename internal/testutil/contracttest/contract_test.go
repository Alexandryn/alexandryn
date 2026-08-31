package contracttest_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil/contracttest"
)

// fakeT is a minimal testing.TB that records whether Error/Errorf/Fatal
// was called, letting the outer test assert that a validator failure was
// triggered without failing the outer test itself.
type fakeT struct {
	testing.TB
	failed  bool
	t       *testing.T
	cleanup []func()
}

func (f *fakeT) Helper()                          {}
func (f *fakeT) Log(args ...any)                  {}
func (f *fakeT) Logf(format string, args ...any)  {}
func (f *fakeT) Error(args ...any)                { f.failed = true }
func (f *fakeT) Errorf(format string, _ ...any)   { f.failed = true }
func (f *fakeT) Fatal(args ...any)                { f.failed = true; panic("fakeT.Fatal") }
func (f *fakeT) Fatalf(format string, _ ...any)   { f.failed = true; panic("fakeT.Fatalf") }
func (f *fakeT) Cleanup(fn func())                { f.cleanup = append(f.cleanup, fn) }

// mustRequest builds an *http.Request, failing the test on error.
func mustRequest(t *testing.T, method, path string, body io.Reader) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestSpecLoadsAndIsValid proves the spec file is reachable and passes
// OpenAPI 3.x validation. This is the minimal "does the contract test
// infrastructure itself work" check (L02).
func TestSpecLoadsAndIsValid(t *testing.T) {
	v := contracttest.New(t)
	doc := v.Doc()
	if doc == nil {
		t.Fatal("expected non-nil doc")
	}

	// Confirm all Phase 06 and Phase 07 paths are present in the spec (L01 + L02
	// cross-check). A missing path means openapi.yaml was not updated.
	phasePaths := []string{
		"/api/v1/library",
		"/api/v1/works/{id}",
		"/api/v1/collections",
		"/api/v1/collections/{id}",
		"/api/v1/collections/{id}/works",
		"/api/v1/collections/{id}/works/{workId}",
		"/api/v1/discover",
		"/api/v1/discover/works/{openLibraryId}",
		"/api/v1/discover/covers/{coverId}",
	}
	for _, p := range phasePaths {
		if doc.Paths.Find(p) == nil {
			t.Errorf("spec missing path: %s", p)
		}
	}
}

// TestBrokenHandlerFailsContractTest proves — in the negative direction —
// that ValidateResponse catches a handler returning a schema-violating
// body (backend-library-api.md FR-8: "the contract test fails on a
// deliberately malformed handler response").
//
// Uses fakeT to intercept the validation failure so it doesn't propagate
// as an outer-test failure — the outer test asserts the inner failure
// occurred.
func TestBrokenHandlerFailsContractTest(t *testing.T) {
	v := contracttest.New(t)

	// Handler returns 200 with an empty JSON object, which is NOT a valid
	// LibraryPage (missing required 'works' and 'nextCursor' fields).
	badHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	inner := &fakeT{t: t}
	req := mustRequest(t, "GET", "/api/v1/library", nil)

	// ValidateResponse will call inner.Errorf when the schema mismatch is
	// detected. We recover from any panic fakeT.Fatal/Fatalf might cause.
	func() {
		defer func() { recover() }() //nolint:errcheck
		v.ValidateResponse(inner, badHandler, req)
	}()

	if !inner.failed {
		t.Error("expected contract validation to detect schema mismatch for {} body, but it did not")
	}
}

// TestUndocumentedRouteNotInSpec proves that a path not present in
// api/openapi.yaml is detectable via Doc().Paths.Find (the
// route-completeness direction of FR-8's dual check). This does not hit
// an actual handler — it only inspects the spec's path set.
func TestUndocumentedRouteNotInSpec(t *testing.T) {
	v := contracttest.New(t)
	doc := v.Doc()

	// /api/v1/nonexistent is not a real endpoint and should not appear.
	if doc.Paths.Find("/api/v1/nonexistent") != nil {
		t.Error("spec unexpectedly contains /api/v1/nonexistent — spec may have stale content")
	}
}

// TestValidLibraryPagePassesContractTest proves the happy path: a
// correctly shaped LibraryPage response passes validation without error.
func TestValidLibraryPagePassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	validBody := `{"works":[],"nextCursor":null}`
	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	})

	req := mustRequest(t, "GET", "/api/v1/library", nil)
	// If the validator calls t.Error, this outer test fails — which is
	// exactly what we want: a valid response must not trigger an error.
	// The body is consumed by the validator, so we only check status.
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestValidDiscoverSearchPassesContractTest proves that a correctly shaped
// NormalisedSearchResponse passes contract validation.
func TestValidDiscoverSearchPassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	validBody := `{
		"items": [
			{
				"openLibraryWorkKey": "OL82563W",
				"title": "Middlemarch",
				"authors": [
					{
						"openLibraryAuthorKey": "OL21594A",
						"name": "George Eliot"
					}
				],
				"firstPublishYear": 1871,
				"coverUrl": "/api/v1/discover/covers/8256301",
				"editionCount": 42
			}
		],
		"total": 1,
		"limit": 20,
		"offset": 0
	}`
	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	})

	req := mustRequest(t, "GET", "/api/v1/discover?q=middlemarch", nil)
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestValidDiscoverWorkDetailPassesContractTest proves that a correctly shaped
// DiscoverWorkDetail passes contract validation.
func TestValidDiscoverWorkDetailPassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	validBody := `{
		"work": {
			"title": "Middlemarch",
			"subtitle": "A Study of Provincial Life",
			"description": "A novel by George Eliot.",
			"subjects": ["Fiction"],
			"authors": [
				{
					"openLibraryAuthorKey": "OL21594A",
					"name": "George Eliot"
				}
			],
			"coverUrl": "/api/v1/discover/covers/8256301"
		},
		"editions": [
			{
				"title": "Middlemarch",
				"publisher": "Penguin Classics",
				"publishDate": "2003",
				"language": "en",
				"openLibraryEditionKey": "OL7353617M",
				"coverUrl": "/api/v1/discover/covers/8256301"
			}
		]
	}`
	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validBody))
	})

	req := mustRequest(t, "GET", "/api/v1/discover/works/OL82563W", nil)
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestValidDiscoverCoverPassesContractTest proves that a correctly shaped
// cover binary image response with caching headers passes contract validation.
func TestValidDiscoverCoverPassesContractTest(t *testing.T) {
	v := contracttest.New(t)

	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Metadata-Cache", "hit")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00"))
	})

	req := httptest.NewRequest("GET", "/api/v1/discover/covers/8256301", nil)
	rr := v.ValidateResponse(t, goodHandler, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}



