package config_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// Phase 13 Tier 0 (backend-network-transport.md FR-2, ADR 0028 §1):
// BIND_ADDRESS classification is stated, not left implicit.
//   - IP literal -> loopback/private check; else public.
//   - "localhost" -> private.
//   - any other host string -> public, WITHOUT DNS resolution.
// Per class:
//   - private: legal; opt-in in-process TLS if TLS_CERT_FILE/KEY_FILE
//     both present (no SAN check); ACME on a private bind is an error.
//   - public: ServeTLS mandatory; accepted with a valid static cert
//     (SAN must include a named host); ACME issuance lands in Tier 2.

func setBind(t *testing.T, addr string) {
	t.Helper()
	validEnv(t)
	t.Setenv("BIND_ADDRESS", addr)
}

func withCert(t *testing.T, certPEM, keyPEM []byte) func(string) ([]byte, error) {
	t.Helper()
	t.Setenv("TLS_CERT_FILE", "/tls/cert.pem")
	t.Setenv("TLS_KEY_FILE", "/tls/key.pem")
	return mapReadFile(map[string][]byte{"/tls/cert.pem": certPEM, "/tls/key.pem": keyPEM})
}

func TestBind_PublicIP_WithValidStaticCert_Accepted(t *testing.T) {
	setBind(t, "203.0.113.5:8443")
	certPEM, keyPEM := validCert(t)
	rf := withCert(t, certPEM, keyPEM)

	if _, err := config.Load("", rf, fakeUserConfigDir); err != nil {
		t.Fatalf("Load() error = %v, want nil — a public bind with a valid static cert is Mode A", err)
	}
}

func TestBind_PublicDNSName_Classification_NoResolution(t *testing.T) {
	// A DNS name is classified public without any lookup; with no cert
	// and no ACME it is refused.
	setBind(t, "library.example.com:443")
	if _, err := config.Load("", noFile, fakeUserConfigDir); err == nil {
		t.Fatal("Load() error = nil, want an error — a DNS-name bind is public and has no certificate")
	} else if !strings.Contains(err.Error(), "BIND_ADDRESS") {
		t.Fatalf("error should name BIND_ADDRESS: %v", err)
	}
}

func TestBind_PublicDNSName_CertSANMustMatch(t *testing.T) {
	t.Run("SAN includes the bind host -> accepted", func(t *testing.T) {
		setBind(t, "library.example.com:443")
		certPEM, keyPEM := certWithSAN(t, "library.example.com")
		rf := withCert(t, certPEM, keyPEM)
		if _, err := config.Load("", rf, fakeUserConfigDir); err != nil {
			t.Fatalf("Load() error = %v, want nil — cert SAN matches the named host", err)
		}
	})
	t.Run("SAN does not include the bind host -> rejected", func(t *testing.T) {
		setBind(t, "library.example.com:443")
		certPEM, keyPEM := certWithSAN(t, "other.example.com")
		rf := withCert(t, certPEM, keyPEM)
		if _, err := config.Load("", rf, fakeUserConfigDir); err == nil {
			t.Fatal("Load() error = nil, want an error — the cert's SANs do not cover the bind host")
		}
	})
}

func TestBind_PublicIP_BareIPSkipsSANCheck(t *testing.T) {
	// A bare-IP public bind validates expiry + key-match only; a
	// name-match check on an IP is meaningless.
	setBind(t, "203.0.113.5:8443")
	certPEM, keyPEM := certWithSAN(t, "unrelated.example.com")
	rf := withCert(t, certPEM, keyPEM)
	if _, err := config.Load("", rf, fakeUserConfigDir); err != nil {
		t.Fatalf("Load() error = %v, want nil — a bare-IP public bind does not do a SAN name check", err)
	}
}

func TestBind_Public_ACMEEnabled_NotYetSupported(t *testing.T) {
	// Tier 2 wires ACME issuance. Until then, ACME_ENABLED on a public
	// bind is a startup error (fail closed), not a plaintext fallback.
	setBind(t, "library.example.com:443")
	t.Setenv("ACME_ENABLED", "true")
	t.Setenv("ACME_DOMAIN", "library.example.com")
	if _, err := config.Load("", noFile, fakeUserConfigDir); err == nil {
		t.Fatal("Load() error = nil, want an error — in-process ACME is not wired until phase 13 Tier 2")
	} else if !strings.Contains(strings.ToLower(err.Error()), "acme") {
		t.Fatalf("error should mention ACME: %v", err)
	}
}

func TestBind_PrivateIP_WithOptInCert_Accepted(t *testing.T) {
	setBind(t, "192.168.1.10:8443")
	certPEM, keyPEM := validCert(t)
	rf := withCert(t, certPEM, keyPEM)
	if _, err := config.Load("", rf, fakeUserConfigDir); err != nil {
		t.Fatalf("Load() error = %v, want nil — opt-in in-process TLS on a private bind (ADR 0028 §1)", err)
	}
}

func TestBind_PrivateIP_WithInvalidOptInCert_Rejected(t *testing.T) {
	setBind(t, "192.168.1.10:8443")
	certPEM, keyPEM := expiredCert(t)
	rf := withCert(t, certPEM, keyPEM)
	if _, err := config.Load("", rf, fakeUserConfigDir); err == nil {
		t.Fatal("Load() error = nil, want an error — a present-but-invalid cert on a private bind is a mistake, not a plaintext fall-through")
	}
}

func TestBind_PrivateIP_ACMEEnabled_IsAConflict(t *testing.T) {
	setBind(t, "192.168.1.10:8443")
	t.Setenv("ACME_ENABLED", "true")
	if _, err := config.Load("", noFile, fakeUserConfigDir); err == nil {
		t.Fatal("Load() error = nil, want an error — ACME HTTP-01 cannot validate a private address")
	} else if !strings.Contains(strings.ToLower(err.Error()), "acme") {
		t.Fatalf("error should mention ACME: %v", err)
	}
}

func TestBind_Unspecified_ClassifiesPublic_FailClosed(t *testing.T) {
	// D-B (tasks/plan-phase13-network.md): 0.0.0.0 / :: bind every
	// interface, public ones included. The classifier has no
	// all-interfaces carve-out — an unspecified address is neither
	// loopback nor a private range, so it classifies public and, with no
	// certificate, is refused. Writing the specific private interface IP
	// for a LAN deployment is the (deferred) first-run flow's job, not a
	// looser classifier here.
	for _, addr := range []string{"0.0.0.0:8080", "[::]:8080"} {
		t.Run(addr, func(t *testing.T) {
			setBind(t, addr)
			if _, err := config.Load("", noFile, fakeUserConfigDir); err == nil {
				t.Fatalf("Load() error = nil for %q, want an error — an unspecified address is public and has no certificate", addr)
			} else if !strings.Contains(err.Error(), "BIND_ADDRESS") {
				t.Fatalf("error should name BIND_ADDRESS: %v", err)
			}
		})
	}
}

func TestBind_Unspecified_WithValidCert_Accepted(t *testing.T) {
	// The other half of D-B: 0.0.0.0 is Mode A, so a valid static cert
	// makes it legal (and cmd/server then serves it over TLS). Bare-IP,
	// so no SAN name check.
	setBind(t, "0.0.0.0:8443")
	certPEM, keyPEM := validCert(t)
	rf := withCert(t, certPEM, keyPEM)
	if _, err := config.Load("", rf, fakeUserConfigDir); err != nil {
		t.Fatalf("Load() error = %v, want nil — 0.0.0.0 is Mode A and the static cert is valid", err)
	}
}

func TestBind_LoopbackAndPrivate_NoCert_StillAccepted(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:0", "[::1]:0", "localhost:8080", "10.0.0.5:8080", "[fc00::1]:8080"} {
		t.Run(addr, func(t *testing.T) {
			setBind(t, addr)
			if _, err := config.Load("", noFile, fakeUserConfigDir); err != nil {
				t.Fatalf("Load() error = %v, want nil for %q", err, addr)
			}
		})
	}
}
