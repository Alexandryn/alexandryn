package config_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// Tests verifying that BIND_ADDRESS is classified into loopback/private
// (permitted for plaintext) or publicly routable (requires valid TLS certificate).

func TestLoad_BindAddress_LoopbackAndPrivateAlwaysAccepted(t *testing.T) {
	cases := []string{
		"127.0.0.1:0",
		"127.0.0.1:8080",
		"[::1]:0",
		"localhost:8080",
		"192.168.1.5:8080", // RFC 1918
		"10.0.0.5:8080",    // RFC 1918
		"[fc00::1]:8080",   // IPv6 unique local
	}
	for _, addr := range cases {
		t.Run(addr, func(t *testing.T) {
			validEnv(t)
			t.Setenv("BIND_ADDRESS", addr)

			if _, err := config.Load("", noFile, fakeUserConfigDir); err != nil {
				t.Fatalf("Load() error = %v, want nil for loopback/private address %q", err, addr)
			}
		})
	}
}

func TestLoad_BindAddress_PubliclyRoutableWithNoCertRejected(t *testing.T) {
	cases := []string{
		"0.0.0.0:8080",
		"203.0.113.5:8080", // TEST-NET-3, publicly routable per IANA allocation
	}
	for _, addr := range cases {
		t.Run(addr, func(t *testing.T) {
			validEnv(t)
			t.Setenv("BIND_ADDRESS", addr)

			_, err := config.Load("", noFile, fakeUserConfigDir)
			if err == nil {
				t.Fatalf("Load() error = nil, want an error — %q is publicly routable with no certificate configured", addr)
			}
			if !strings.Contains(err.Error(), "BIND_ADDRESS") && !strings.Contains(err.Error(), "TLS_CERT_FILE") {
				t.Fatalf("error %q doesn't name the offending condition", err.Error())
			}
		})
	}
}

func TestLoad_BindAddress_UnrecognizedHostRejected(t *testing.T) {
	// A hostname that isn't "localhost" or a literal IP is classified
	// as publicly routable without DNS lookup; with no certificate configured it is rejected.
	validEnv(t)
	t.Setenv("BIND_ADDRESS", "example.com:8080")

	_, err := config.Load("", noFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error — a DNS-name bind is public and has no certificate")
	}
	if !strings.Contains(err.Error(), "BIND_ADDRESS") {
		t.Fatalf("error %q doesn't name BIND_ADDRESS", err.Error())
	}
}

func TestLoad_BindAddress_MalformedRejected(t *testing.T) {
	validEnv(t)
	t.Setenv("BIND_ADDRESS", "not-a-host-port")

	_, err := config.Load("", noFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error for a BIND_ADDRESS with no host:port syntax")
	}
	if !strings.Contains(err.Error(), "BIND_ADDRESS") {
		t.Fatalf("error %q doesn't name BIND_ADDRESS", err.Error())
	}
}

func TestLoad_BindAddress_PubliclyRoutableWithInvalidCertRejected(t *testing.T) {
	cases := []struct {
		name string
		cert func(t *testing.T) (certPEM, keyPEM []byte)
	}{
		{"expired certificate", expiredCert},
		{"mismatched key", func(t *testing.T) ([]byte, []byte) {
			cert1, _ := validCert(t)
			_, key2 := validCert(t)
			return cert1, key2
		}},
		{"malformed PEM", func(t *testing.T) ([]byte, []byte) {
			return []byte("not a certificate"), []byte("not a key")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			t.Setenv("BIND_ADDRESS", "203.0.113.5:8080")
			t.Setenv("TLS_CERT_FILE", "/tls/cert.pem")
			t.Setenv("TLS_KEY_FILE", "/tls/key.pem")

			certPEM, keyPEM := tc.cert(t)
			readFile := mapReadFile(map[string][]byte{
				"/tls/cert.pem": certPEM,
				"/tls/key.pem":  keyPEM,
			})

			_, err := config.Load("", readFile, fakeUserConfigDir)
			if err == nil {
				t.Fatalf("Load() error = nil, want an error — a public bind must fail startup on an invalid certificate, never degrade to unencrypted (%s)", tc.name)
			}
		})
	}
}

func TestLoad_BindAddress_PubliclyRoutableWithMissingCertFileRejected(t *testing.T) {
	validEnv(t)
	t.Setenv("BIND_ADDRESS", "203.0.113.5:8080")
	t.Setenv("TLS_CERT_FILE", "/tls/does-not-exist.pem")
	t.Setenv("TLS_KEY_FILE", "/tls/does-not-exist-key.pem")

	_, err := config.Load("", noFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error — the configured certificate file doesn't exist")
	}
}

func TestLoad_BindAddress_DefaultIsLoopback(t *testing.T) {
	validEnv(t)

	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.BindAddress != "127.0.0.1:0" {
		t.Fatalf("BindAddress = %q, want the compiled default %q", cfg.BindAddress, "127.0.0.1:0")
	}
}
