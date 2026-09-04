//go:build integration

package http_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"golang.org/x/crypto/acme"
)

// TestACME_IssuanceAndRenewal_AgainstPebble exercises the real ACME
// certificate lifecycle the phase-13 exit criterion requires — issuance
// on first handshake, then a forced near-expiry renewal — against Pebble
// (Let's Encrypt's test ACME server) plus pebble-challtestsrv for DNS.
//
// Env (set by ci.yml's backend job, or a local `podman` run):
//
//	PEBBLE_DIRECTORY_URL   e.g. https://localhost:14000/dir
//	PEBBLE_HTTP_PORT       the port Pebble's VA connects back on for HTTP-01
//	PEBBLE_TEST_DOMAIN     a name challtestsrv resolves to this host
//
// Local verification 2026-09-04: with this exact NewACMEManager config,
// Pebble validates the HTTP-01 challenge and issues a certificate end to
// end ("authz set VALID" -> "Issued certificate serial ..."). The final
// GetCertificate call then fails on cert *download* — `Post "": unsupported
// protocol scheme ""` — an incompatibility between x/crypto v0.54.0's acme
// client (autocert reading the finalized order's `certificate` URL) and
// this Pebble image's finalize response. Getting the assertion green needs
// a compatible Pebble pin or an x/crypto bump; tracked as a phase-13
// follow-up. The production wiring (HostPolicy, GetCertificate, the :80
// HTTPHandler) is unit-tested in acme_test.go and proven at the Pebble
// level here.
func TestACME_IssuanceAndRenewal_AgainstPebble(t *testing.T) {
	dirURL := os.Getenv("PEBBLE_DIRECTORY_URL")
	httpPort := os.Getenv("PEBBLE_HTTP_PORT")
	domain := os.Getenv("PEBBLE_TEST_DOMAIN")
	if dirURL == "" || httpPort == "" || domain == "" {
		t.Skip("Pebble not configured (PEBBLE_DIRECTORY_URL / PEBBLE_HTTP_PORT / PEBBLE_TEST_DOMAIN) — skipping the ACME lifecycle test")
	}

	// A client that trusts Pebble's ephemeral CA. Pebble ships its root at
	// /roots/0 on the management interface, but for the ACME transport we
	// only need to skip verification of Pebble's own directory TLS.
	acmeHTTP := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // Pebble test CA
	}

	cfg := &config.Config{ACMEDomain: domain, ACMEEmail: "ci@example.test"}
	cacheDir := t.TempDir()
	m := transporthttp.NewACMEManager(cfg, cacheDir)
	m.Client = &acme.Client{DirectoryURL: dirURL, HTTPClient: acmeHTTP}

	// stripPort delegates to the autocert challenge handler with the port
	// removed from Host. In production HTTP-01 is always on port 80 so the
	// Host header is a bare domain and autocert.HostWhitelist matches it;
	// Pebble's VA calls back on a non-standard port here, so the shim
	// keeps the whitelist check meaningful. Test-only.
	stripPort := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h, _, err := net.SplitHostPort(r.Host); err == nil {
				r.Host = h
			}
			next.ServeHTTP(w, r)
		})
	}

	// Serve the HTTP-01 challenge on the port Pebble's VA calls back on.
	challLn, lnErr := net.Listen("tcp", ":"+httpPort)
	if lnErr != nil {
		t.Fatalf("bind challenge port %s: %v", httpPort, lnErr)
	}
	challSrv := &http.Server{Handler: stripPort(m.HTTPHandler(nil)), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = challSrv.Serve(challLn) }()
	defer challSrv.Close()

	issue := func() *tls.Certificate {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cert, err := m.GetCertificate(&tls.ClientHelloInfo{ServerName: domain, Conn: fakeConn{}})
		_ = ctx
		if err != nil {
			t.Fatalf("GetCertificate: %v", err)
		}
		leaf, perr := x509.ParseCertificate(cert.Certificate[0])
		if perr != nil {
			t.Fatalf("parse issued leaf: %v", perr)
		}
		if err := leaf.VerifyHostname(domain); err != nil {
			t.Fatalf("issued cert does not cover %q: %v", domain, err)
		}
		return cert
	}

	first := issue()
	firstLeaf, _ := x509.ParseCertificate(first.Certificate[0])

	// Force a renewal: autocert renews when the cached cert is within its
	// RenewBefore window of NotAfter. Set RenewBefore past the cert's whole
	// lifetime and clear the in-memory cache entry so the next
	// GetCertificate re-issues.
	m.RenewBefore = time.Until(firstLeaf.NotAfter) + time.Hour
	// autocert has no exported cache-clear; a fresh Manager over the same
	// DirCache reads the cached cert, sees it "expiring", and re-issues.
	m2 := transporthttp.NewACMEManager(cfg, cacheDir)
	m2.Client = m.Client
	m2.RenewBefore = m.RenewBefore
	_ = challSrv.Close()
	challLn2, _ := net.Listen("tcp", ":"+httpPort)
	challSrv2 := &http.Server{Handler: stripPort(m2.HTTPHandler(nil)), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = challSrv2.Serve(challLn2) }()
	defer challSrv2.Close()
	time.Sleep(100 * time.Millisecond)

	renewed, err := m2.GetCertificate(&tls.ClientHelloInfo{ServerName: domain, Conn: fakeConn{}})
	if err != nil {
		t.Fatalf("renewal GetCertificate: %v", err)
	}
	renewedLeaf, _ := x509.ParseCertificate(renewed.Certificate[0])
	if renewedLeaf.SerialNumber.Cmp(firstLeaf.SerialNumber) == 0 {
		t.Fatal("renewal returned the same certificate serial — no new issuance happened")
	}
}

type fakeConn struct{ net.Conn }

func (fakeConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (fakeConn) RemoteAddr() net.Addr { return &net.TCPAddr{} }
