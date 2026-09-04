package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"time"
)

// bindClass is BIND_ADDRESS's classification per ADR 0028 §1 / ADR 0017.
type bindClass int

const (
	// classLoopbackPrivate — Mode B: TLS, if any, terminates upstream or
	// via an opt-in in-process certificate; the process is never itself
	// directly reachable from a public address.
	classLoopbackPrivate bindClass = iota
	// classPublic — Mode A: in-process TLS is mandatory.
	classPublic
)

// classifyBindHost classifies BIND_ADDRESS's host WITHOUT any DNS
// resolution (architecture-testing.md FR-6 — this package performs no
// network I/O). An IP literal is classified by range; the literal string
// "localhost" is private; every other host string is a DNS name and is
// classified public — the fail-closed default for "a name we cannot
// classify locally is one that requires in-process TLS." An operator who
// runs a name behind a reverse proxy and wants Mode B sets BIND_ADDRESS
// to the private IP the proxy forwards to, not the name.
func classifyBindHost(host string) bindClass {
	if host == "localhost" {
		return classLoopbackPrivate
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return classPublic
	}
	// net.IP.IsPrivate covers RFC 1918 IPv4 and fc00::/7 (IPv6 ULA).
	// Link-local (169.254.0.0/16, fe80::/10) is also a local-only range —
	// an operator on an interface with no DHCP lease binds there, and it
	// is never publicly routable.
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return classLoopbackPrivate
	}
	return classPublic
}

// validateBindAddress enforces ADR 0028 §1 / ADR 0017 at config-load
// time. It fails closed: a publicly routable bind with no usable
// certificate never produces a running configuration, and an
// invalid/expired certificate is a startup error, never a degrade to
// plaintext.
//
// Phase 13 Tier 0 scope: the static-certificate path (both public and the
// private opt-in) is fully validated and accepted here. In-process ACME
// issuance is Tier 2 — until then, ACME_ENABLED on a public bind is a
// startup error rather than a plaintext fallback.
func validateBindAddress(cfg *Config, readFile func(string) ([]byte, error)) error {
	host, _, err := net.SplitHostPort(cfg.BindAddress)
	if err != nil {
		return fmt.Errorf("BIND_ADDRESS must be host:port: %w", err)
	}

	// A half-configured pair is a mistake on any bind class, never a
	// silent fall-through to plaintext: on a private bind it would
	// otherwise skip the opt-in TLS branch below and serve HTTP; on a
	// public bind the "cert required" error would fire but misleadingly
	// name both files as missing when one is set.
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		setKey := "TLS_KEY_FILE"
		if cfg.TLSCertFile != "" {
			setKey = "TLS_CERT_FILE"
		}
		return fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be set together (only %s is set)", setKey)
	}
	hasStaticCert := cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""

	switch classifyBindHost(host) {
	case classLoopbackPrivate:
		if cfg.ACMEEnabled {
			return fmt.Errorf(
				"BIND_ADDRESS %s is a loopback/private address; ACME_ENABLED cannot apply — an ACME HTTP-01 challenge needs a publicly reachable address. Use a static TLS_CERT_FILE/TLS_KEY_FILE, or terminate TLS with a reverse proxy in front",
				cfg.BindAddress)
		}
		if hasStaticCert {
			// Opt-in in-process TLS on a private bind (ADR 0028 §1): a
			// present pair must be valid; a present-but-broken cert is a
			// mistake, not a fall-through to plaintext. No SAN check — a
			// private bind is often reached by IP.
			cert, err := loadAndValidateCert(cfg, readFile, "")
			if err != nil {
				return err
			}
			cfg.tlsCert = cert
		}
		return nil

	case classPublic:
		if cfg.ACMEEnabled {
			return fmt.Errorf(
				"BIND_ADDRESS %s is publicly routable with ACME_ENABLED, but in-process ACME issuance is not wired yet (phase 13 Tier 2). Configure a static TLS_CERT_FILE/TLS_KEY_FILE for now",
				cfg.BindAddress)
		}
		if !hasStaticCert {
			return fmt.Errorf(
				"BIND_ADDRESS %s is not a loopback or private-range address, so in-process TLS is required: set TLS_CERT_FILE and TLS_KEY_FILE, or bind to a loopback/private address behind a reverse proxy",
				cfg.BindAddress)
		}
		// SAN name check only when the host is a DNS name — a name-match
		// check on a bare IP is meaningless (ADR 0028 §1).
		sanHost := ""
		if net.ParseIP(host) == nil {
			sanHost = host
		}
		cert, err := loadAndValidateCert(cfg, readFile, sanHost)
		if err != nil {
			return err
		}
		cfg.tlsCert = cert
		return nil

	default:
		// Unreachable today (classifyBindHost returns one of the two
		// constants above). Kept as a fail-closed guard: this is the
		// constitution §6 gate, and a future third class must not slip
		// through as "accepted" by falling off the switch.
		return fmt.Errorf("BIND_ADDRESS %s could not be classified", cfg.BindAddress)
	}
}

// loadAndValidateCert loads TLS_CERT_FILE/TLS_KEY_FILE through the
// injected readFile (never a direct filesystem call) and checks the
// certificate is well-formed, its key matches, and it is within its
// validity window. When sanHost is non-empty the leaf must cover it. The
// returned *tls.Certificate is the serving material cmd/server wraps the
// listener with (Config.TLSCertificate).
func loadAndValidateCert(cfg *Config, readFile func(string) ([]byte, error), sanHost string) (*tls.Certificate, error) {
	certPEM, err := readFile(cfg.TLSCertFile)
	if err != nil {
		return nil, fmt.Errorf("could not read TLS_CERT_FILE %s: %w", cfg.TLSCertFile, err)
	}
	keyPEM, err := readFile(cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read TLS_KEY_FILE %s: %w", cfg.TLSKeyFile, err)
	}

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("TLS_CERT_FILE/TLS_KEY_FILE are invalid: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return nil, errors.New("TLS_CERT_FILE contains no certificate")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("TLS_CERT_FILE could not be parsed: %w", err)
	}

	now := time.Now()
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return nil, fmt.Errorf("TLS certificate is not currently valid (validity window %s to %s)", leaf.NotBefore, leaf.NotAfter)
	}

	if sanHost != "" {
		if err := leaf.VerifyHostname(sanHost); err != nil {
			return nil, fmt.Errorf("TLS certificate does not cover BIND_ADDRESS host %q: %w", sanHost, err)
		}
	}

	pair.Leaf = leaf
	return &pair, nil
}
