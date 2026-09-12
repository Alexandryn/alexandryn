package http

import (
	"net/http"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// categoryStatus is the category-to-HTTP-status mapping.
// Total and fixed, every domain category maps to exactly one status.
// This is the single source of truth; nothing else in this codebase duplicates it.
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
// produce — defaults to Internal's status, ensuring no unmapped status codes.
func StatusForCategory(category domain.Category) int {
	if status, ok := categoryStatus[category]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Limits rejects a request whose body is at or over maxBodyBytes with an
// InvalidInput response before the handler ever runs, and wraps the body
// with http.MaxBytesReader for defense-in-depth against a request whose
// true size isn't known upfront (chunked transfer, or an inaccurate
// Content-Length header).
func Limits(maxBodyBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength >= 0 && r.ContentLength >= maxBodyBytes {
				WriteError(w, domain.InvalidInput,
					"request body exceeds the maximum allowed size", CorrelationIDFromContext(r.Context()))
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// NotFoundHandler returns the shared NotFound JSON shape,
// registered as /api/v1/...'s own catch-all so an unmatched path never
// falls through to ServeMux's default plain-text 404.
func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, domain.NotFound,
			"no such endpoint", CorrelationIDFromContext(r.Context()))
	})
}
