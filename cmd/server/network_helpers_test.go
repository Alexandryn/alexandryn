package main

import (
	"net"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/transport/mdns"
)

// TestScopeForIP_LinkLocalIsNotPublic reproduces the bug where interface
// scope classification used `!ip.IsPrivate()` -> "public", which misses
// link-local addresses (169.254.0.0/16, fe80::/10) since Go's
// net.IP.IsPrivate() does not cover RFC 3927/link-local ranges. An
// interface with only a link-local address (no DHCP lease) was reported as
// "public" on the network-status admin screen — a misleading signal on a
// security-relevant status display.
func TestScopeForIP_LinkLocalIsNotPublic(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		want string
	}{
		{"private IPv4 (RFC 1918)", "192.168.1.5", "lan"},
		{"link-local IPv4", "169.254.1.1", "lan"},
		{"link-local IPv6", "fe80::1", "lan"},
		{"public IPv4", "203.0.113.9", "public"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) failed", tc.ip)
			}
			if got := scopeForIP(ip); got != tc.want {
				t.Errorf("scopeForIP(%s) = %q, want %q", tc.ip, got, tc.want)
			}
		})
	}
}

// TestResolveServerAddress_NonWildcardBindPassesThrough covers the common
// case (an explicit LAN bind) unaffected by the wildcard bug: the address
// is already dialable, so it should pass through unchanged.
func TestResolveServerAddress_NonWildcardBindPassesThrough(t *testing.T) {
	cfg := &config.Config{BindAddress: "10.0.0.5:8443"}
	if got := resolveServerAddress(cfg); got != "10.0.0.5:8443" {
		t.Errorf("resolveServerAddress = %q, want 10.0.0.5:8443", got)
	}
}

// TestResolveServerAddress_WildcardBindNeverReturnsWildcardHost reproduces
// the bug where NetworkAPI.ServerAddress was set to the raw cfg.BindAddress
// with no wildcard exclusion: BIND_ADDRESS=0.0.0.0:8443 (a legitimate LAN
// bind) produced a pairing QR/deep-link payload and status Address of
// literally "0.0.0.0:8443" — not a routable destination from another host,
// silently breaking pairing whenever the operator uses a wildcard bind.
func TestResolveServerAddress_WildcardBindNeverReturnsWildcardHost(t *testing.T) {
	for _, bind := range []string{"0.0.0.0:8443", "[::]:8443"} {
		cfg := &config.Config{BindAddress: bind}
		got := resolveServerAddress(cfg)
		if strings.HasPrefix(got, "0.0.0.0") || strings.HasPrefix(got, "[::]") || strings.HasPrefix(got, "::") {
			t.Errorf("resolveServerAddress(%q) = %q, still an undialable wildcard host", bind, got)
		}
	}
}

// TestComputeAllowedOrigins_NoDuplicates guards the shared
// resolveBindAddress refactor: a CORS_ALLOWED_ORIGINS entry that coincides
// with the derived bind-address origin must not be listed twice.
func TestComputeAllowedOrigins_NoDuplicates(t *testing.T) {
	cfg := &config.Config{
		BindAddress:        "10.0.0.5:8443",
		CORSAllowedOrigins: []string{"http://10.0.0.5:8443"},
	}
	origins := computeAllowedOrigins(cfg)
	seen := make(map[string]int)
	for _, o := range origins {
		seen[o]++
	}
	for o, n := range seen {
		if n > 1 {
			t.Errorf("origin %q appears %d times in computeAllowedOrigins, want deduplicated", o, n)
		}
	}
}

func TestComputeAllowedOrigins_LoopbackIncludesLocalhostAnd127(t *testing.T) {
	cfg := &config.Config{
		BindAddress: "127.0.0.1:33879",
	}
	origins := computeAllowedOrigins(cfg)
	originSet := make(map[string]bool)
	for _, o := range origins {
		originSet[o] = true
	}

	for _, want := range []string{
		"http://127.0.0.1:33879",
		"http://localhost:33879",
		"http://[::1]:33879",
		"http://alexandryn.local:33879",
	} {
		if !originSet[want] {
			t.Errorf("expected origin %q to be allowed for loopback bind, got: %v", want, origins)
		}
	}
}

func TestComputeAllowedOrigins_WildcardIncludesLoopback(t *testing.T) {
	cfg := &config.Config{
		BindAddress: "0.0.0.0:8080",
	}
	origins := computeAllowedOrigins(cfg)
	originSet := make(map[string]bool)
	for _, o := range origins {
		originSet[o] = true
	}

	for _, want := range []string{
		"http://127.0.0.1:8080",
		"http://localhost:8080",
		"http://[::1]:8080",
		"http://alexandryn.local:8080",
	} {
		if !originSet[want] {
			t.Errorf("expected origin %q to be allowed for wildcard bind, got: %v", want, origins)
		}
	}
}

func TestFallbackNetworkInfo_LoopbackOmitAlexandrynLocal(t *testing.T) {
	cfg := &config.Config{
		BindAddress: "127.0.0.1:8080",
	}
	info := fallbackNetworkInfo(cfg)()
	if info.Reachability != "loopback" {
		t.Errorf("Reachability = %q, want loopback", info.Reachability)
	}
	if info.HostnameVerified {
		t.Errorf("HostnameVerified = true on loopback bind, want false")
	}
	for _, addr := range info.Addresses {
		if strings.Contains(addr.URL, "alexandryn.local") {
			t.Errorf("found alexandryn.local in loopback addresses: %v", info.Addresses)
		}
	}
}

func TestFallbackNetworkInfo_LANIncludesAlexandrynLocal(t *testing.T) {
	cfg := &config.Config{
		BindAddress: "0.0.0.0:8080",
	}
	info := fallbackNetworkInfo(cfg)()
	if info.Reachability != "lan" {
		t.Errorf("Reachability = %q, want lan", info.Reachability)
	}
	if info.HostnameVerified {
		t.Errorf("HostnameVerified = true without verified mDNS, want false")
	}
	found := false
	for _, addr := range info.Addresses {
		if strings.Contains(addr.URL, "alexandryn.local:8080") {
			found = true
			if addr.Scope != "lan" {
				t.Errorf("alexandryn.local scope = %q, want lan", addr.Scope)
			}
		}
	}
	if !found {
		t.Errorf("expected alexandryn.local:8080 in LAN addresses, got: %v", info.Addresses)
	}
}

func TestFallbackNetworkInfo_HostnameVerifiedWithMDNS(t *testing.T) {
	cfg := &config.Config{
		BindAddress: "0.0.0.0:8080",
	}
	ms, err := mdns.NewServer(mdns.Config{Hostname: "alexandryn.local", Port: 8080})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	ms.SetVerified(true)

	info := fallbackNetworkInfo(cfg, ms)()
	if !info.HostnameVerified {
		t.Errorf("HostnameVerified = false with verified mDNS server, want true")
	}

	ms.SetVerified(false)
	infoUnverified := fallbackNetworkInfo(cfg, ms)()
	if infoUnverified.HostnameVerified {
		t.Errorf("HostnameVerified = true with unverified mDNS server, want false")
	}
}
