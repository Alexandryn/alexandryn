package http

import (
	"encoding/json"
	"net/http"

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
// unauthenticated public surface once the bind opens beyond loopback.
// Tier 4 adds separate, stricter PublicRateLimit layers for the pairing
// routes.
//
// The SPA's embedded static assets are deliberately NOT rate-limited
// here: they are served from an in-memory embed.FS (no DB, no compute),
// a flood of them is bandwidth only (the excluded DoS category), and
// behind a reverse proxy every client shares one RemoteAddr — a tight
// bucket there would 429 a legitimate multi-user SPA load mid-boot. The
// health probes are the real anonymous-hammer target and cost a pool
// ping, so they keep a bucket.
//
// The client IP comes from clientIP: the connection's RemoteAddr, unless
// RemoteAddr is a configured trusted proxy (TRUSTED_PROXY_CIDRS), in
// which case the last X-Forwarded-For hop. An unconfigured deployment
// trusts no proxy and ignores the header entirely (#195).
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

// HealthProbePath reports whether path is one of the two unauthenticated
// health probes — FR-8's first rate-limit bucket.
func HealthProbePath(path string) bool {
	return path == "/healthz" || path == "/readyz"
}
