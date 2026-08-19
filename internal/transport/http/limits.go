package http

import (
	"net/http"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// categoryStatus is the category-to-HTTP-status mapping,
// backend-errors-and-logging.md FR-2 — total and fixed, every category
// maps to exactly one status. This is the one place it lives; nothing
// else in this codebase should duplicate it.
var categoryStatus = map[domain.Category]int{
	domain.NotFound:     http.StatusNotFound,
	domain.InvalidInput: http.StatusBadRequest,
	domain.Unauthorized: http.StatusUnauthorized,
	domain.Conflict:     http.StatusConflict,
	domain.Unavailable:  http.StatusServiceUnavailable,
	domain.Internal:     http.StatusInternalServerError,
}

// StatusForCategory returns category's HTTP status. An unrecognized
// category — which domain.Category's closed set should never actually
// produce — defaults to Internal's status, the same "never an unmapped
// status" property FR-4 requires on the input side.
func StatusForCategory(category domain.Category) int {
	if status, ok := categoryStatus[category]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Limits rejects a request whose body is at or over maxBodyBytes with an
// InvalidInput response before the handler ever runs, and wraps the body
// with http.MaxBytesReader for defense-in-depth against a request whose
// true size isn't known upfront (chunked transfer, or a lying
// Content-Length) — backend-http-transport.md FR-2.
func Limits(maxBodyBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength >= 0 && r.ContentLength >= maxBodyBytes {
				writeError(w, StatusForCategory(domain.InvalidInput), domain.InvalidInput,
					"request body exceeds the maximum allowed size", CorrelationIDFromContext(r.Context()))
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// NotFoundHandler returns the shared NotFound JSON shape
// (backend-errors-and-logging.md FR-5) — registered as /api/v1/...'s own
// catch-all so an unmatched path never falls through to ServeMux's
// default plain-text 404 (FR-7).
func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, StatusForCategory(domain.NotFound), domain.NotFound,
			"no such endpoint", CorrelationIDFromContext(r.Context()))
	})
}
