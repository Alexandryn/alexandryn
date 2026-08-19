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
// TLS or none, outside this process's guarantee); a publicly routable
// address is FR-8's Mode A (in-process TLS via TLSCertFile/TLSKeyFile) in
// spec text, but is rejected outright here regardless of certificate
// validity — cmd/server does not yet call ServeTLS anywhere, so Mode A's
// "legal" outcome would otherwise mean Load succeeding while the process
// silently serves plaintext HTTP on a public address. Validating a
// certificate that's never used to actually encrypt anything is worse
// than no validation, since it looks enforced but isn't (Checkpoint F
// security review, T17-T19). Revert to calling
// validatePublicBindCertificate once ServeTLS is actually wired
// (phase 13) — that function is kept, tested, and ready for that switch.
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

	if err := validatePublicBindCertificate(cfg, readFile); err != nil {
		return err
	}
	return fmt.Errorf("BIND_ADDRESS %s is publicly routable; this build does not yet serve TLS (no ServeTLS wiring exists), so public binds are refused regardless of certificate validity until that lands", cfg.BindAddress)
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
