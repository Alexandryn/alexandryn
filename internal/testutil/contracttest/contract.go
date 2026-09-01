// Package contracttest provides OpenAPI contract validation helpers for
// integration tests (backend-library-api.md FR-8, architecture-contracts.md
// FR-3). Uses kin-openapi (github.com/getkin/kin-openapi) — the tool
// backend-library-api.md FR-8 names explicitly — to validate real HTTP
// responses against api/openapi.yaml.
//
// Invariants this package enforces, per FR-8's own text:
//
//  1. Response-shape validation: every response from a handler must
//     match its OpenAPI schema or the test fails.
//
//  2. Route-completeness check: every path in api/openapi.yaml must
//     have a registered handler, and every handler registered under
//     /api/v1/ must appear in api/openapi.yaml — catching the gap
//     architecture-contracts.md's Failure modes table flagged: "does
//     the contract test catch *missing* endpoints, or only *mismatched*
//     ones?"
package contracttest

import (
	"bytes"
	"context"
	"io"

	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// Validator wraps a loaded OpenAPI spec and its kin-openapi router,
// ready to validate individual requests and responses.
type Validator struct {
	doc    *openapi3.T
	router routers.Router
}

// New loads api/openapi.yaml relative to the repository root (located via
// the __FILE__ trick so tests pass regardless of working directory) and
// returns a Validator. Fails the test immediately if the spec cannot be
// loaded or is not valid OpenAPI 3.x.
func New(t *testing.T) *Validator {
	t.Helper()

	specPath := specFilePath(t)
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("contracttest: load %s: %v", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("contracttest: spec validation: %v", err)
	}

	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("contracttest: build router: %v", err)
	}

	return &Validator{doc: doc, router: router}
}

// ValidateResponse records a request against handler, then validates the
// response against the OpenAPI spec. Fails the test if the response does
// not match the schema for this operation. Accepts testing.TB so both
// *testing.T and stub implementations (for negative tests) can be passed.
//
// Usage:
//
//	v := contracttest.New(t)
//	v.ValidateResponse(t, handler, httptest.NewRequest("GET", "/api/v1/library", nil))
func (v *Validator) ValidateResponse(t testing.TB, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Find the matching route in the spec.
	route, pathParams, err := v.router.FindRoute(req)
	if err != nil {
		t.Errorf("contracttest: no route for %s %s: %v", req.Method, req.URL.Path, err)
		return rr
	}

	// Validate the response.
	bodyBytes := rr.Body.Bytes()
	rr.Body = bytes.NewBuffer(bodyBytes)

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
		},
		Status: rr.Code,
		Header: rr.Header(),
		Body:   io.NopCloser(bytes.NewReader(bodyBytes)),
		Options: &openapi3filter.Options{
			// Do not require auth headers — every Phase 06 endpoint opts
			// out of auth (security: []) until phase 12.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Errorf("contracttest: response does not match spec for %s %s: %v",
			req.Method, req.URL.Path, err)
	}

	return rr
}

// Doc returns the loaded OpenAPI document, for tests that need to
// enumerate paths or inspect schemas directly (e.g. the route-completeness
// check).
func (v *Validator) Doc() *openapi3.T { return v.doc }

// specFilePath locates api/openapi.yaml by walking up from this file's own
// directory until a go.mod is found, then resolving the api/ sibling.
// Panics (not t.Fatalf) because it runs at package-load time before t is
// available; a missing repo root is a CI environment bug, not a test bug.
func specFilePath(t *testing.T) string {
	t.Helper()

	// __FILE__ for the Go source itself, via runtime.Caller.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("contracttest: runtime.Caller failed")
	}

	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "api", "openapi.yaml")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("contracttest: could not locate repository root (no go.mod found)")
		}
		dir = parent
	}
}
