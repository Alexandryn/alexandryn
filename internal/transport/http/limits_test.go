package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

const oneMiB = 1 << 20

// A body under the limit is read normally; a body at or over it is
// rejected with InvalidInput before the handler ever runs — proven by a
// handler that records whether it ran, not just by the response status.
func TestLimits_BodyUnderLimitPassesThrough(t *testing.T) {
	ran := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
		w.WriteHeader(http.StatusOK)
	})
	handler := transporthttp.Limits(10 * oneMiB)(next)

	body := bytes.Repeat([]byte("a"), oneMiB) // well under 10 MiB
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !ran {
		t.Fatal("handler did not run for a body under the limit")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestLimits_BodyAtOrOverLimitRejectedBeforeHandlerRuns(t *testing.T) {
	cases := []struct {
		name string
		size int64
	}{
		{"exactly at the limit", 10 * oneMiB},
		{"one byte over the limit", 10*oneMiB + 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ran = true
				w.WriteHeader(http.StatusOK)
			})
			handler := transporthttp.Limits(10 * oneMiB)(next)

			body := bytes.Repeat([]byte("a"), int(tc.size))
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if ran {
				t.Fatal("handler ran despite an oversized body")
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (InvalidInput)", rec.Code)
			}

			var body2 struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body2); err != nil {
				t.Fatalf("response body isn't valid JSON: %v", err)
			}
			if body2.Code != "InvalidInput" {
				t.Fatalf("code = %q, want InvalidInput", body2.Code)
			}
		})
	}
}

func TestLimits_JustUnderTheLimitPasses(t *testing.T) {
	ran := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
	})
	handler := transporthttp.Limits(10 * oneMiB)(next)

	body := bytes.Repeat([]byte("a"), 10*oneMiB-1)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !ran {
		t.Fatal("handler did not run for a body one byte under the limit")
	}
}

// --- Category-to-status mapping, total and fixed ---

func TestStatusForCategory_EveryCategoryMapsToExactlyOneStatus(t *testing.T) {
	cases := []struct {
		category domain.Category
		want     int
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
			got := transporthttp.StatusForCategory(tc.category)
			if got != tc.want {
				t.Fatalf("StatusForCategory(%s) = %d, want %d", tc.category, got, tc.want)
			}
		})
	}
}

// An unrecognized category — which domain.Category's closed set should
// never actually produce — still maps to a real status rather than the
// zero value, ensuring an unmapped status code defaults to Internal.
func TestStatusForCategory_UnrecognizedCategoryDefaultsToInternal(t *testing.T) {
	got := transporthttp.StatusForCategory(domain.Category("SomethingNotInTheClosedSet"))
	if got != http.StatusInternalServerError {
		t.Fatalf("StatusForCategory(unrecognized) = %d, want 500", got)
	}
}

// An unmatched /api/v1/... path returns the shared NotFound JSON
// shape, never ServeMux's own default plain-text 404 — proven alongside
// a real registered path to confirm the catch-all doesn't shadow it.
func TestNotFoundHandler_UnmatchedAPIRouteReturnsSharedJSONShape(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/known", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/api/v1/", transporthttp.NotFoundHandler())

	t.Run("unmatched path", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}
		if strings.Contains(rec.Body.String(), "<") {
			t.Fatalf("body looks like ServeMux's own HTML/plain-text 404, not JSON: %s", rec.Body.String())
		}

		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("response body isn't valid JSON: %v\n%s", err, rec.Body.String())
		}
		if body.Code != "NotFound" {
			t.Fatalf("code = %q, want NotFound", body.Code)
		}
	})

	t.Run("registered path still works", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/known", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 — the catch-all must not shadow a real registered route", rec.Code)
		}
	})
}
