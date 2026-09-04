package http

import (
	"net/http"
	"strings"

	"golang.org/x/crypto/acme/autocert"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// NewACMEManager builds the autocert.Manager for a Mode A ACME bind
// (backend-network-transport.md FR-3, ADR 0028 §2). §9 record for
// golang.org/x/crypto/acme/autocert is in ADR 0028 §2 — it is a
// sub-package of golang.org/x/crypto, already a direct dependency
// (Argon2id), so no new module.
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
// https:// form of the same host and path. It runs on the :80 listener
// for every public bind (FR-3) — for a static-cert bind on its own, and
// behind autocert.Manager.HTTPHandler for an ACME bind (which serves the
// HTTP-01 challenge and delegates everything else here). It never serves
// application content.
//
// tlsPort is the port the TLS listener is bound to; it is appended to the
// redirect target unless it is empty or 443.
func HTTPSRedirect(tlsPort string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if i := strings.LastIndexByte(host, ':'); i >= 0 {
			host = host[:i]
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
