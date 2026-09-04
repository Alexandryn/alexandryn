package http

import (
	"net/http"
	"net/url"
	"strings"
)

// OriginValidation wraps the UNAUTHENTICATED state-changing routes — in
// phase 13, POST /api/v1/network/pair/verify (backend-network-transport.md
// FR-7, ADR 0028 §5). Every other state-changing route is already
// Authorization-gated and CSRF-safe by construction (a browser does not
// attach the Bearer header cross-site), so this is a route-group wrapper,
// not a global layer.
//
//   - A request whose Origin header is PRESENT and not in `allowed` -> 403.
//   - A request with NO Origin header -> allowed through. A native client,
//     curl, or a server-to-server call sends none, and Origin absence is
//     not evidence of forgery; browsers always send Origin on a
//     cross-origin fetch/XHR and a cross-origin form POST. The pairing
//     code's single-use + <=5-minute TTL + per-IP rate limit is the real
//     control; this only closes the "hostile page in a victim's browser"
//     path.
//   - Referer is consulted ONLY when Origin is absent AND the request has
//     a body; a present-and-non-matching Referer host is likewise
//     rejected.
//
// `allowed` is computed once at startup by the listener layer as the union
// of every non-loopback interface origin the server serves, the configured
// mDNS host name, and every CORS_ALLOWED_ORIGINS entry — enumerated, so an
// implementer cannot loosen it to a substring match.
func OriginValidation(allowed []string) Middleware {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[strings.ToLower(strings.TrimRight(o, "/"))] = struct{}{}
	}
	inSet := func(origin string) bool {
		_, ok := set[strings.ToLower(strings.TrimRight(origin, "/"))]
		return ok
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corrID := CorrelationIDFromContext(r.Context())

			if origin := r.Header.Get("Origin"); origin != "" {
				if !inSet(origin) {
					writeForbidden(w, "request origin is not allowed", corrID)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// No Origin. Consult Referer only for a request with a body
			// (ContentLength == -1 means chunked with unknown length —
			// that counts as a body).
			if r.ContentLength != 0 {
				if ref := r.Header.Get("Referer"); ref != "" {
					if u, err := url.Parse(ref); err != nil || !originAllowedFromReferer(u, set) {
						writeForbidden(w, "request origin is not allowed", corrID)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// originAllowedFromReferer rebuilds the scheme://host[:port] origin from a
// Referer URL and checks it against the allowed set.
func originAllowedFromReferer(u *url.URL, set map[string]struct{}) bool {
	if u.Scheme == "" || u.Host == "" {
		return false
	}
	origin := strings.ToLower(u.Scheme + "://" + u.Host)
	_, ok := set[origin]
	return ok
}
