package sources_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
)

func TestIsBlockedDialIP(t *testing.T) {
	cases := []struct {
		name         string
		ip           string
		allowPrivate bool
		blocked      bool
	}{
		{"loopback v4", "127.0.0.1", false, true},
		{"loopback v6", "::1", false, true},
		{"unspecified", "0.0.0.0", false, true},
		{"cloud metadata / link-local", "169.254.169.254", false, true},
		{"link-local v6", "fe80::1", false, true},
		{"cgnat", "100.64.1.1", false, true},
		{"multicast", "224.0.0.1", false, true},
		{"rfc1918 blocked by default", "10.1.2.3", false, true},
		{"rfc1918 192.168", "192.168.1.1", false, true},
		{"ula v6", "fd00::1", false, true},
		{"rfc1918 allowed with opt-in", "10.1.2.3", true, false},
		{"loopback allowed with opt-in", "127.0.0.1", true, false},
		{"public still allowed with opt-in", "8.8.8.8", true, false},
		{"public", "8.8.8.8", false, false},
		{"metadata still blocked with opt-in", "169.254.169.254", true, true},
		{"cgnat still blocked with opt-in", "100.64.1.1", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("bad test IP %q", tc.ip)
			}
			if got := sources.IsBlockedDialIP(ip, tc.allowPrivate); got != tc.blocked {
				t.Fatalf("IsBlockedDialIP(%s, allowPrivate=%v) = %v, want %v", tc.ip, tc.allowPrivate, got, tc.blocked)
			}
		})
	}
}

func TestGuardedTransport_RefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("secret internal body"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: sources.GuardedTransport(false), Timeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected the guarded transport to refuse a loopback connection")
	}
	var blocked *sources.BlockedAddressError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected a *BlockedAddressError, got %v", err)
	}
}

func TestGuardedTransport_AllowsPublicResolution(t *testing.T) {
	// A public IP literal must pass the Control hook. Point at a public
	// literal on a port nothing listens on; the dial must fail with a
	// connection error, never a BlockedAddressError.
	client := &http.Client{Transport: sources.GuardedTransport(false), Timeout: 1 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://93.184.216.34:9/", nil)
	_, err := client.Do(req)
	if err == nil {
		return
	}
	var blocked *sources.BlockedAddressError
	if errors.As(err, &blocked) {
		t.Fatalf("public address was wrongly blocked: %v", err)
	}
}
