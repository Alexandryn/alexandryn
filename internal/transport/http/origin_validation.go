package http

import (
	"net/http"
	"net/url"
	"strings"
)

// OriginValidation wraps the UNAUTHENTICATED state-changing routes:
// POST /api/v1/network/pair/verify, and POST /api/v1/auth/setup and /api/v1/auth/login — the
// other two IsPublicPath routes that change state with no Authorization
// header to make them CSRF-safe by construction. Every *authenticated*
// state-changing route doesn't need this: a browser does not attach the
// Bearer header cross-site. This is a route-group wrapper, not a global
// layer, because that authenticated-by-construction argument covers
// everything else.
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
// implementer cannot loosen it to a substring match. Every entry MUST
// already be in the canonical serialized-origin form CORS.go and
// config.parseOriginList use (lower-cased host, default port dropped,
// IPv6 bracketed) — this middleware does exact matching only, the same
// posture CORS takes, rather than re-normalizing with a second, divergent
// implementation that could disagree with CORS's on the same allowlist.
func OriginValidation(allowed []string) Middleware {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	inSet := func(origin string) bool {
		_, ok := set[origin]
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
// Referer URL and checks it against the allowed set. url.Parse already
// lower-cases the scheme; the host is explicitly lower-cased too, to match
// `allowed`'s documented canonical form (CORS/config.parseOriginList
// already lower-case the host) — a browser is free to send a Referer with
// a mixed-case host, and that must compare equal, not spuriously reject a
// legitimate request.
func originAllowedFromReferer(u *url.URL, set map[string]struct{}) bool {
	if u.Scheme == "" || u.Host == "" {
		return false
	}
	_, ok := set[u.Scheme+"://"+strings.ToLower(u.Host)]
	return ok
}
