// Package http implements Alexandryn's HTTP transport: router, middleware,
// handlers, and wire-shape helpers (architecture-backend.md FR-1).
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/observability"
)

// Middleware wraps a handler with another layer of behavior — direct
// function composition (ADR 0011), the mechanism architecture-backend.md
// FR-6's fixed order is built from.
type Middleware func(http.Handler) http.Handler

// Chain composes mw around handler, outermost first: Chain(h, a, b, c)
// serves a request through a, then b, then c, then handler —
// architecture-backend.md FR-6's fixed order (recovery, limits, logging,
// [auth, reserved], routing) is expressed by the order mw is passed in.
func Chain(handler http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		handler = mw[i](handler)
	}
	return handler
}

type correlationIDKey struct{}

// correlationIDHolder is a mutable box for the correlation ID. A plain
// context.WithValue call returns a new, immutable context node — invisible
// to a closure (Recovery's deferred recover) that captured an earlier
// *http.Request before an inner middleware (Logging) attached the ID.
// Mutating a holder already reachable through the shared context, instead
// of shadowing it with a new value, is what lets an outer layer observe
// an inner layer's assignment after the fact.
type correlationIDHolder struct{ id string }

// WithCorrelationID attaches id as the request's correlation ID. If ctx
// already carries a holder, it's mutated in place; otherwise a new one
// is installed. Recovery installs an empty holder before calling into
// the rest of the chain specifically so that a later call — from
// Logging, or from any inner middleware — mutates that same holder
// rather than being invisible to Recovery's own already-captured
// request.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	if h, ok := ctx.Value(correlationIDKey{}).(*correlationIDHolder); ok {
		h.id = id
		return ctx
	}
	return context.WithValue(ctx, correlationIDKey{}, &correlationIDHolder{id: id})
}

// CorrelationIDFromContext returns the correlation ID currently set on
// ctx's holder, or "" if none is set yet (a panic before logging ran —
// Recovery generates a fallback for exactly this case) or no holder
// exists at all.
func CorrelationIDFromContext(ctx context.Context) string {
	h, ok := ctx.Value(correlationIDKey{}).(*correlationIDHolder)
	if !ok {
		return ""
	}
	return h.id
}

type errorBody struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlationId"`
}

// WriteError is the one shared helper every error response goes through
// (backend-errors-and-logging.md FR-5): it maps category to its HTTP
// status (StatusForCategory) and writes architecture-contracts.md FR-5's
// wire shape as JSON. No handler or middleware builds an error body by
// hand.
func WriteError(w http.ResponseWriter, category domain.Category, message, correlationID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(StatusForCategory(category))
	_ = json.NewEncoder(w).Encode(errorBody{
		Code:          string(category),
		Message:       message,
		CorrelationID: correlationID,
	})
}

// Recovery is the outermost middleware (FR-1): it catches a panic from
// any inner layer, including routing and handlers, logs it server-side
// with a stack trace, and responds with a generic Internal error via the
// shared response helper — never the panic's own message or a stack
// trace to the client (backend-errors-and-logging.md FR-10). If the
// request context doesn't yet carry a correlation ID (a panic in a layer
// that runs before logging), Recovery generates a fallback itself so the
// response's correlationId is never empty regardless of which layer
// panicked.
func Recovery(logger *slog.Logger, newID func() string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(WithCorrelationID(r.Context(), ""))

			defer func() {
				rec := recover()
				if rec == nil {
					return
				}

				id := CorrelationIDFromContext(r.Context())
				if id == "" {
					id = newID()
				}

				logger.Error("panic recovered",
					"correlationId", id,
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()),
				)

				WriteError(w, domain.Internal, "an internal error occurred", id)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Logging generates the correlation ID (backend-errors-and-logging.md
// FR-7), attaches it to the request's context, and logs a debug-level
// line at request start and an info-level line at completion (method,
// path, status code, duration) — both carrying the same ID (FR-4).
func Logging(logger *slog.Logger, newID func() string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := newID()
			r = r.WithContext(WithCorrelationID(r.Context(), id))

			logger.Debug("request started",
				"correlationId", id,
				"method", r.Method,
				"path", r.URL.Path,
			)

			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			duration := time.Since(start)

			logger.Info("request completed",
				"correlationId", id,
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration", duration.String(),
			)
		})
	}
}

// Metrics records request latency per route template in the given registry (FR-2).
// Static assets under /assets/* are ignored.
func Metrics(reg *observability.Registry) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if reg == nil || strings.HasPrefix(r.URL.Path, "/assets/") {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)

			route := r.Pattern
			if route == "" {
				route = r.URL.Path
			}
			reg.ObserveRequest(route, duration)
		})
	}
}

