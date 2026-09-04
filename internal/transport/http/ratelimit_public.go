package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/auth"
)

// writeRateLimited writes a 429 with the shared error-body shape. Like
// writeForbidden, the domain taxonomy has no RateLimited category — 429 is
// a transport-owned outcome (backend-network-transport.md FR-8).
func writeRateLimited(w http.ResponseWriter, corrID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(errorBody{
		Code:          "RateLimited",
		Message:       "too many requests, slow down and try again shortly",
		CorrelationID: corrID,
	})
}

// PublicRateLimit applies a per-client-IP token bucket to every request
// `applies` returns true for (backend-network-transport.md FR-8) — the
// unauthenticated public surface that is broader than a LAN-only threat
// model once the bind opens: /healthz, /readyz, the embedded static
// assets, and (added in Tier 4) the pairing routes, each with its own
// bucket via a separate PublicRateLimit layer.
//
// The client IP is taken from the connection's RemoteAddr, NEVER from
// X-Forwarded-For — trusting a client-supplied header for rate-limit
// keying is itself the bypass. A trusted-proxy configuration that would
// make XFF safe does not exist in phase 13.
//
// A request that does not match `applies` passes straight through, so this
// composes with phase 12's auth-endpoint limiter rather than
// double-counting it.
func PublicRateLimit(limiter *auth.IPRateLimiter, applies func(path string) bool) Middleware {
	return func(next http.Handler) http.Handler {
		if limiter == nil || applies == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if applies(r.URL.Path) && !limiter.Allow(clientIP(r)) {
				writeRateLimited(w, CorrelationIDFromContext(r.Context()))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HealthAndStaticPath reports whether path is part of the health / static
// public surface FR-8's first bucket covers: the two health probes and
// anything that is not an /api/v1 route (the SPA and its assets).
func HealthAndStaticPath(path string) bool {
	switch path {
	case "/healthz", "/readyz":
		return true
	}
	return !strings.HasPrefix(path, "/api/v1/")
}
