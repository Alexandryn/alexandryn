package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"golang.org/x/crypto/acme/autocert"
)

func TestHTTPSRedirect(t *testing.T) {
	cases := []struct {
		name, canonical, tlsPort, host, path, want string
	}{
		{"canonical host, default port", "books.example.com", "443", "anything", "/x?y=1", "https://books.example.com/x?y=1"},
		{"canonical host, non-default port", "books.example.com", "8443", "anything", "/a", "https://books.example.com:8443/a"},
		{"no canonical -> request host, port stripped", "", "8443", "192.168.1.24:80", "/a", "https://192.168.1.24:8443/a"},
		{"forged Host is ignored when canonical is set", "books.example.com", "443", "evil.example", "/a", "https://books.example.com/a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "http://"+tc.host+tc.path, nil)
			r.Host = tc.host
			transporthttp.HTTPSRedirect(tc.canonical, tc.tlsPort).ServeHTTP(rec, r)

			if rec.Code != http.StatusPermanentRedirect {
				t.Fatalf("status = %d, want 308", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != tc.want {
				t.Fatalf("Location = %q, want %q", loc, tc.want)
			}
		})
	}
}

func TestNewACMEManager_HostPolicyPinnedToTheDomain(t *testing.T) {
	cfg := &config.Config{ACMEDomain: "books.example.com", ACMEEmail: "ops@example.com"}
	m := transporthttp.NewACMEManager(cfg, t.TempDir())

	if err := m.HostPolicy(nil, "books.example.com"); err != nil {
		t.Fatalf("HostPolicy rejected the configured domain: %v", err)
	}
	if err := m.HostPolicy(nil, "evil.example.com"); err == nil {
		t.Fatal("HostPolicy accepted a domain other than ACME_DOMAIN — issuance amplification")
	}
	if m.Prompt == nil {
		t.Fatal("Prompt must be set (AcceptTOS) or autocert cannot register an account")
	}
	if _, ok := m.Cache.(autocert.DirCache); !ok {
		t.Fatalf("Cache = %T, want autocert.DirCache", m.Cache)
	}
	if m.Email != "ops@example.com" {
		t.Errorf("Email = %q", m.Email)
	}
}
