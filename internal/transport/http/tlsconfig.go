package http

import "crypto/tls"

// NewTLSConfig returns the version / cipher / ALPN policy every in-process
// TLS listener uses (backend-network-transport.md FR-4, ADR 0028 §3). The
// caller sets exactly one certificate source on the result — Certificates
// for a static pair, or GetCertificate for an autocert.Manager.
//
//   - MinVersion TLS 1.2; TLS 1.3 is negotiated when the client offers it.
//   - For TLS 1.2, an explicit AEAD + ECDHE (forward-secret) cipher list —
//     no CBC, so no padding-oracle family. Go ignores CipherSuites for
//     TLS 1.3, whose suites are all safe and not configurable.
//   - ALPN offers h2 then http/1.1; net/http serves HTTP/2 automatically
//     over TLS.
//
// PreferServerCipherSuites is deliberately left unset — Go 1.17+ ignores
// it and orders by the client's hardware AES support, which is better.
func NewTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		NextProtos: []string{"h2", "http/1.1"},
	}
}
