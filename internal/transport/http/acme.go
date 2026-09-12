package http

import (
	"net"
	"net/http"
	"strings"

	"golang.org/x/crypto/acme/autocert"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// NewACMEManager builds the autocert.Manager for an ACME TLS configuration.
// It uses golang.org/x/crypto/acme/autocert from the existing golang.org/x/crypto dependency.
//
//   - HostPolicy is pinned to exactly ACME_DOMAIN: a client presenting
//     any other SNI gets no certificate and no issuance attempt. This is
//     what stops the server doing ACME work on an attacker's behalf.
//   - Prompt = AcceptTOS: enabling ACME accepts the CA's Terms of Service
//     on the operator's behalf. cmd/server logs the CA directory URL at
//     startup so the operator knows what was agreed to (§11/§12).
//   - Cache is a DirCache at cacheDir; autocert creates it 0700 and the
//     account/certificate keys inside are secrets on disk.
func NewACMEManager(cfg *config.Config, cacheDir string) *autocert.Manager {
	return &autocert.Manager{
		Cache:      autocert.DirCache(cacheDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.ACMEDomain),
		Email:      cfg.ACMEEmail,
	}
}

// HTTPSRedirect returns a handler that 308-redirects every request to the
// https:// form of the same path. It runs on the :80 listener for every
// public bind — for a static-cert bind on its own, and behind
// autocert.Manager.HTTPHandler for an ACME bind (which serves the HTTP-01
// challenge and delegates everything else here). It never serves
// application content.
//
//   - canonicalHost, when non-empty (ACME_DOMAIN, or a named BIND_ADDRESS
//     host), is the redirect target's host — the request's own Host
//     header is not trusted for it. When empty (a bare-IP public bind)
//     the request Host is the only option; the port is stripped.
//   - tlsPort is appended unless it is empty or 443.
func HTTPSRedirect(canonicalHost, tlsPort string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := canonicalHost
		if host == "" {
			// net.SplitHostPort, not a bare LastIndexByte(':') split — a
			// bracketed IPv6 literal has a ':' before the closing bracket
			// that a naive split would cut at, mangling the redirect
			// target. SplitHostPort strips the brackets on success, so a
			// host containing ':' (IPv6) needs re-bracketing for the URL.
			if h, _, err := net.SplitHostPort(r.Host); err == nil {
				host = h
			} else {
				host = r.Host // no port present — r.Host is already the bare (possibly bracketed) host
			}
			if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
				host = "[" + host + "]"
			}
		}
		target := "https://" + host
		if tlsPort != "" && tlsPort != "443" {
			target += ":" + tlsPort
		}
		target += r.URL.RequestURI()
		w.Header().Set("Connection", "close")
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	})
}
