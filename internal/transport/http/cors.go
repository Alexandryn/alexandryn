package http

import (
	"net/http"
	"strings"
)

// CORS is deny-by-default (backend-network-transport.md FR-6, ADR 0028
// §4). It is a no-op for the default deployment: the Go server serves the
// SPA, so browser clients are same-origin and never exercise CORS.
//
//   - allowedOrigins empty  -> never emit any Access-Control-Allow-*
//     header; answer an OPTIONS preflight 204 with no CORS headers, so a
//     cross-origin browser request fails the browser's own check.
//   - allowedOrigins present -> echo Access-Control-Allow-Origin ONLY on a
//     byte-for-byte exact Origin match (no scheme fold, no port wildcard,
//     no suffix match). Config already normalized the entries to the
//     serialized-origin form (config.parseOriginList).
//
// Access-Control-Allow-Credentials is NEVER set — the API is
// Bearer-authenticated, not cookie-authenticated, so it is not needed and
// the credentials+wildcard combination is a known bypass shape.
//
// The one real cross-origin case this supports: a reverse proxy
// terminating TLS on a hostname the Go process does not know itself by.
func CORS(allowedOrigins []string) Middleware {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			_, ok := allowed[origin]
			if origin != "" && ok {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Library-Id")
				h.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions && strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")) != "" {
				// A CORS preflight. With no matching origin this is an
				// empty 204 (effectively a non-CORS OPTIONS); with a match
				// the headers above are already set.
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
