package config_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// Phase 13 (backend-configuration.md FR-4 amendment, ADR 0028) adds six
// keys: ACME_ENABLED, ACME_DOMAIN, ACME_EMAIL, ACME_CACHE_DIR,
// CORS_ALLOWED_ORIGINS, DEVICE_PAIRING_SECRET.

func TestLoad_ACMEEnabled_DefaultsFalse(t *testing.T) {
	validEnv(t)
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ACMEEnabled {
		t.Fatal("ACMEEnabled = true, want false by default")
	}
}

func TestLoad_ACMEEnabled_Parsing(t *testing.T) {
	// The default bind is loopback, where ACME_ENABLED=true is a
	// classification conflict (validateBindAddress). So a truthy value
	// reaches bind validation and fails there with an ACME message
	// (proving it parsed); a falsey value lets Load succeed; a
	// non-boolean is a parse error naming the key.
	for _, tc := range []struct {
		raw            string
		wantParsedTrue bool // true -> reaches an ACME conflict; false -> Load ok
		parseErr       bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"1", true, false},
		{"0", false, false},
		{"TRUE", true, false},
		{"yes", false, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			validEnv(t)
			t.Setenv("ACME_ENABLED", tc.raw)
			_, err := config.Load("", noFile, fakeUserConfigDir)

			switch {
			case tc.parseErr:
				if err == nil || !strings.Contains(err.Error(), "ACME_ENABLED") {
					t.Fatalf("ACME_ENABLED=%q: want a parse error naming ACME_ENABLED, got %v", tc.raw, err)
				}
			case tc.wantParsedTrue:
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), "acme") {
					t.Fatalf("ACME_ENABLED=%q: want an ACME bind conflict (proving it parsed true), got %v", tc.raw, err)
				}
			default:
				if err != nil {
					t.Fatalf("ACME_ENABLED=%q: want Load to succeed (parsed false), got %v", tc.raw, err)
				}
			}
		})
	}
}

func TestLoad_ACMEStringKeys_OptionalNoDefault(t *testing.T) {
	validEnv(t)
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ACMEDomain != "" || cfg.ACMEEmail != "" || cfg.ACMECacheDir != "" {
		t.Fatalf("ACME string keys should be empty when unset: domain=%q email=%q cache=%q",
			cfg.ACMEDomain, cfg.ACMEEmail, cfg.ACMECacheDir)
	}

	t.Setenv("ACME_DOMAIN", "library.example.com")
	t.Setenv("ACME_EMAIL", "ops@example.com")
	t.Setenv("ACME_CACHE_DIR", "/var/lib/alexandryn/acme")
	cfg, err = config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ACMEDomain != "library.example.com" || cfg.ACMEEmail != "ops@example.com" || cfg.ACMECacheDir != "/var/lib/alexandryn/acme" {
		t.Fatalf("ACME string keys did not round-trip: %+v", cfg)
	}
}

func TestLoad_CORSAllowedOrigins_ParsesAndValidates(t *testing.T) {
	validEnv(t)

	// Unset -> empty.
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("CORSAllowedOrigins = %v, want empty when unset", cfg.CORSAllowedOrigins)
	}

	// A well-formed comma list, with surrounding whitespace.
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://a.example , https://b.example:8443 ")
	cfg, err = config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"https://a.example", "https://b.example:8443"}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[0] != want[0] || cfg.CORSAllowedOrigins[1] != want[1] {
		t.Fatalf("CORSAllowedOrigins = %v, want %v", cfg.CORSAllowedOrigins, want)
	}

	// Entries are normalized to the serialized-origin form a browser
	// sends (RFC 6454 §6.1): host lower-cased, the scheme's default port
	// dropped, a non-default port kept.
	for _, tc := range []struct{ in, want string }{
		{"HTTPS://App.Example:8443", "https://app.example:8443"},
		{"https://library.example.com:443", "https://library.example.com"},
		{"http://host.example:80", "http://host.example"},
		{"https://host.example:8443", "https://host.example:8443"},
		{"HTTP://Host.Example", "http://host.example"},
	} {
		validEnv(t)
		t.Setenv("CORS_ALLOWED_ORIGINS", tc.in)
		cfg, err = config.Load("", noFile, fakeUserConfigDir)
		if err != nil {
			t.Fatalf("Load() %q error = %v", tc.in, err)
		}
		if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != tc.want {
			t.Fatalf("CORS_ALLOWED_ORIGINS=%q -> %v, want [%s]", tc.in, cfg.CORSAllowedOrigins, tc.want)
		}
	}

	for _, bad := range []string{
		"a.example",                  // no scheme
		"https://a.example/callback", // path present
		"ftp://a.example",            // wrong scheme
		"https://",                   // no host
	} {
		t.Run("rejects "+bad, func(t *testing.T) {
			validEnv(t)
			t.Setenv("CORS_ALLOWED_ORIGINS", bad)
			if _, err := config.Load("", noFile, fakeUserConfigDir); err == nil {
				t.Fatalf("CORS_ALLOWED_ORIGINS=%q: want a validation error, got nil", bad)
			} else if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
				t.Fatalf("error should name CORS_ALLOWED_ORIGINS: %v", err)
			}
		})
	}
}

func TestLoad_DevicePairingSecret_IsRedacted(t *testing.T) {
	validEnv(t)
	t.Setenv("DEVICE_PAIRING_SECRET", "s3cr3t-pairing-value")
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DevicePairingSecret.Reveal() != "s3cr3t-pairing-value" {
		t.Fatalf("Reveal() = %q, want the real value", cfg.DevicePairingSecret.Reveal())
	}
	if strings.Contains(cfg.DevicePairingSecret.String(), "s3cr3t") {
		t.Fatalf("String() leaked the secret: %q", cfg.DevicePairingSecret.String())
	}
	j, _ := json.Marshal(cfg.DevicePairingSecret)
	if strings.Contains(string(j), "s3cr3t") {
		t.Fatalf("MarshalJSON leaked the secret: %s", j)
	}
}

func TestLoad_DevicePairingSecret_AbsentIsEmpty(t *testing.T) {
	validEnv(t)
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DevicePairingSecret.Reveal() != "" {
		t.Fatalf("DevicePairingSecret should be empty when unset, got %q", cfg.DevicePairingSecret.Reveal())
	}
}
