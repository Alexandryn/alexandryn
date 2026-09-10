package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// #119: a handler that falls through to WriteError with err.Error() leaks
// server-only text (raw driver errors, internal paths) when the error is
// not a *domain.Error. Every such fall-through must go through
// writeDomainError, which collapses a non-domain error to the generic
// "unexpected error". One representative handler per file that had the
// leak.
func TestHandlers_DoNotLeakRawErrorText(t *testing.T) {
	const rawErr = `pq: duplicate key value violates unique constraint "x_pkey" (/home/user/db)`

	assertGeneric := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		var body struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body not JSON: %v\n%s", err, rec.Body.String())
		}
		if body.Code != string(domain.Internal) {
			t.Errorf("code = %q, want Internal", body.Code)
		}
		if body.Message != "unexpected error" {
			t.Errorf("message = %q, want %q", body.Message, "unexpected error")
		}
		if strings.Contains(rec.Body.String(), "pq:") || strings.Contains(rec.Body.String(), "constraint") || strings.Contains(rec.Body.String(), "/home/") {
			t.Errorf("raw error text leaked into response: %s", rec.Body.String())
		}
	}

	t.Run("collections.go ListCollectionsHandler", func(t *testing.T) {
		repo := &mockCollectionRepository{
			findAllFunc: func(context.Context) ([]*domain.CollectionSummary, error) {
				return nil, errors.New(rawErr)
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/collections", nil)
		req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), domain.DefaultLibraryID))
		rec := httptest.NewRecorder()
		transporthttp.ListCollectionsHandler(repo).ServeHTTP(rec, req)
		assertGeneric(t, rec)
	})

	t.Run("sources.go ListSourcesHandler", func(t *testing.T) {
		repo := newMockSourceRecordRepo()
		repo.listFunc = func(context.Context) ([]postgres.SourceRecord, error) { return nil, errors.New(rawErr) }
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
		rec := httptest.NewRecorder()
		transporthttp.ListSourcesHandler(repo).ServeHTTP(rec, req)
		assertGeneric(t, rec)
	})

	t.Run("library.go LibraryHandler", func(t *testing.T) {
		repo := &mockWorkRepository{
			queryLibraryFunc: func(context.Context, domain.LibraryQuery) (*domain.LibraryPage, error) {
				return nil, errors.New(rawErr)
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/works", nil)
		req = withLibraryUser(req, testLib)
		rec := httptest.NewRecorder()
		transporthttp.LibraryHandler(repo).ServeHTTP(rec, req)
		assertGeneric(t, rec)
	})
}
