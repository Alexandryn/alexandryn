package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// bindClass is BIND_ADDRESS's classification.
type bindClass int

const (
	// classLoopback / classPrivate — Mode B: TLS, if any, terminates
	// upstream or via an opt-in in-process certificate; the process is
	// never itself directly reachable from a public address.
	classLoopback bindClass = iota
	classPrivate
	// classPublic — Mode A: in-process TLS is mandatory and a :80
	// HTTP->HTTPS redirect listener runs.
	classPublic
)

func (c bindClass) reachability() string {
	switch c {
	case classLoopback:
		return "loopback"
	case classPrivate:
		return "private"
	default:
		return "public"
	}
}

// classifyBindHost classifies BIND_ADDRESS's host without performing DNS resolution.
// An IP literal is classified by range; the literal string "localhost" is loopback;
// every other host string is treated as a DNS name and classified as public (fail-closed).
func classifyBindHost(host string) bindClass {
	if host == "localhost" {
		return classLoopback
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return classPublic
	}
	if ip.IsLoopback() {
		return classLoopback
	}
	// net.IP.IsPrivate covers RFC 1918 IPv4 and fc00::/7 (IPv6 ULA).
	// Link-local (169.254.0.0/16, fe80::/10) is also a local-only range —
	// an operator on an interface with no DHCP lease binds there, and it
	// is never publicly routable.
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return classPrivate
	}
	return classPublic
}

// validateBindAddress enforces TLS requirements for the bind address at config-load
// time. It fails closed: a publicly routable bind with no usable
// certificate never produces a running configuration, and an
// invalid/expired certificate is a startup error, never a degrade to
// plaintext.
//
// It also records cfg.reachability and cfg.tlsMode from the resolved
// class and the certificate/ACME state, for cmd/server's listener
// selection and /network/status.
func validateBindAddress(cfg *Config, readFile func(string) ([]byte, error)) error {
	host, _, err := net.SplitHostPort(cfg.BindAddress)
	if err != nil {
		return fmt.Errorf("BIND_ADDRESS must be host:port: %w", err)
	}

	class := classifyBindHost(host)
	cfg.reachability = class.reachability()
	cfg.tlsMode = "none"

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

	switch class {
	case classLoopback, classPrivate:
		if cfg.ACMEEnabled {
			return fmt.Errorf(
				"BIND_ADDRESS %s is a loopback/private address; ACME_ENABLED cannot apply — an ACME HTTP-01 challenge needs a publicly reachable address. Use a static TLS_CERT_FILE/TLS_KEY_FILE, or terminate TLS with a reverse proxy in front",
				cfg.BindAddress)
		}
		if hasStaticCert {
			// Opt-in in-process TLS on a private bind: a present pair
			// must be valid; an invalid certificate is a configuration error.
			// No SAN check is performed because private binds are commonly addressed by IP.
			cert, err := loadAndValidateCert(cfg, readFile, "")
			if err != nil {
				return err
			}
			cfg.tlsCert = cert
			cfg.tlsMode = "static"
		}
		return nil

	case classPublic:
		// A DNS name, not a bare IP literal — computed once and reused
		// below (the ACME domain-match check, the SAN-check gate, and
		// cfg.namedBindHost, which cmd/server reads instead of
		// re-deriving this same classification for the :80 redirect).
		isName := net.ParseIP(host) == nil
		if isName {
			cfg.namedBindHost = host
		}
		if cfg.ACMEEnabled {
			// In-process TLS via ACME. config performs no network
			// I/O — it only checks that the configuration is coherent.
			if cfg.ACMEDomain == "" {
				return fmt.Errorf("BIND_ADDRESS %s has ACME_ENABLED but ACME_DOMAIN is not set", cfg.BindAddress)
			}
			if isName && !strings.EqualFold(host, cfg.ACMEDomain) {
				return fmt.Errorf("BIND_ADDRESS host %q must equal ACME_DOMAIN %q", host, cfg.ACMEDomain)
			}
			if hasStaticCert {
				return fmt.Errorf("ACME_ENABLED and TLS_CERT_FILE/TLS_KEY_FILE are mutually exclusive — pick one certificate source")
			}
			cfg.tlsMode = "acme"
			return nil
		}
		if !hasStaticCert {
			return fmt.Errorf(
				"BIND_ADDRESS %s is not a loopback or private-range address, so in-process TLS is required: set TLS_CERT_FILE and TLS_KEY_FILE, enable ACME_ENABLED with an ACME_DOMAIN, or bind to a loopback/private address behind a reverse proxy",
				cfg.BindAddress)
		}
		// SAN name check only when the host is a DNS name — a name-match
		// check on a bare IP is meaningless.
		sanHost := ""
		if isName {
			sanHost = host
		}
		cert, err := loadAndValidateCert(cfg, readFile, sanHost)
		if err != nil {
			return err
		}
		cfg.tlsCert = cert
		cfg.tlsMode = "static"
		return nil

	default:
		// Fall-through guard: ensures unclassified addresses cannot pass.
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
