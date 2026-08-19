package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// Checkpoint E's own smoke test: all six domain.Error categories through
// WriteError, end to end, in one place — not inferred from the fact that
// three of them happen to be exercised individually by Recovery,
// NotFoundHandler, and Readyz elsewhere in this package.
func TestWriteError_AllSixCategories(t *testing.T) {
	cases := []struct {
		category domain.Category
		status   int
	}{
		{domain.NotFound, http.StatusNotFound},
		{domain.InvalidInput, http.StatusBadRequest},
		{domain.Unauthorized, http.StatusUnauthorized},
		{domain.Conflict, http.StatusConflict},
		{domain.Unavailable, http.StatusServiceUnavailable},
		{domain.Internal, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(string(tc.category), func(t *testing.T) {
			rec := httptest.NewRecorder()
			transporthttp.WriteError(rec, tc.category, "something went wrong", "corr-id-123")

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}

			var body struct {
				Code          string `json:"code"`
				Message       string `json:"message"`
				CorrelationID string `json:"correlationId"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body isn't valid JSON: %v\n%s", err, rec.Body.String())
			}
			if body.Code != string(tc.category) {
				t.Fatalf("code = %q, want %q", body.Code, tc.category)
			}
			if body.Message != "something went wrong" {
				t.Fatalf("message = %q, want the given message", body.Message)
			}
			if body.CorrelationID != "corr-id-123" {
				t.Fatalf("correlationId = %q, want corr-id-123", body.CorrelationID)
			}
		})
	}
}
