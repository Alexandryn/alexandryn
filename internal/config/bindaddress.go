package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"time"
)

// validateBindAddress enforces FR-8/ADR 0017: BindAddress's resolved host
// must be loopback or a private range (always legal, Mode B — upstream
// TLS or none, outside this process's guarantee), or publicly routable
// with TLSCertFile/TLSKeyFile both present and valid (Mode A — in-process
// TLS). An invalid or missing certificate on a public bind fails startup
// unconditionally; it never degrades to an unencrypted listener.
func validateBindAddress(cfg *Config, readFile func(string) ([]byte, error)) error {
	host, _, err := net.SplitHostPort(cfg.BindAddress)
	if err != nil {
		return fmt.Errorf("BIND_ADDRESS must be host:port: %w", err)
	}

	private, err := isLoopbackOrPrivate(host)
	if err != nil {
		return fmt.Errorf("BIND_ADDRESS host is invalid: %w", err)
	}
	if private {
		return nil
	}

	return validatePublicBindCertificate(cfg, readFile)
}

// isLoopbackOrPrivate classifies host without a real DNS lookup: only a
// literal IP address or the literal string "localhost" can be classified
// at all — any other hostname is rejected outright, since this package
// never performs network I/O to resolve one (architecture-testing.md
// FR-6's determinism requirement, restated at the config layer).
func isLoopbackOrPrivate(host string) (bool, error) {
	if host == "localhost" {
		return true, nil
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false, fmt.Errorf(`must be an IP address or "localhost" (got %q) — this package never performs a DNS lookup to classify a hostname`, host)
	}

	return ip.IsLoopback() || ip.IsPrivate(), nil
}

// validatePublicBindCertificate loads and validates TLSCertFile/
// TLSKeyFile through the same injected readFile the config file uses —
// never a direct filesystem call — and checks the certificate is
// well-formed, its key matches, and it's within its validity window.
func validatePublicBindCertificate(cfg *Config, readFile func(string) ([]byte, error)) error {
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
		return fmt.Errorf("BIND_ADDRESS %s is publicly routable; TLS_CERT_FILE and TLS_KEY_FILE are required", cfg.BindAddress)
	}

	certPEM, err := readFile(cfg.TLSCertFile)
	if err != nil {
		return fmt.Errorf("could not read TLS_CERT_FILE %s: %w", cfg.TLSCertFile, err)
	}
	keyPEM, err := readFile(cfg.TLSKeyFile)
	if err != nil {
		return fmt.Errorf("could not read TLS_KEY_FILE %s: %w", cfg.TLSKeyFile, err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("TLS_CERT_FILE/TLS_KEY_FILE are invalid: %w", err)
	}

	if len(cert.Certificate) == 0 {
		return errors.New("TLS_CERT_FILE contains no certificate")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("TLS_CERT_FILE could not be parsed: %w", err)
	}

	now := time.Now()
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return fmt.Errorf("TLS certificate is not currently valid (validity window %s to %s)", leaf.NotBefore, leaf.NotAfter)
	}

	return nil
}
