package openlibrary

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBlockedResolvedIP(t *testing.T) {
	cases := map[string]bool{
		"93.184.216.34":   false, // public
		"1.1.1.1":         false,
		"127.0.0.1":       true, // loopback
		"10.0.0.5":        true, // RFC 1918
		"192.168.1.1":     true,
		"169.254.169.254": true, // link-local / cloud metadata
		"100.64.0.1":      true, // CGNAT
		"::1":             true, // loopback v6
		"fe80::1":         true, // link-local v6
		"0.0.0.0":         true, // unspecified
	}
	for ipStr, want := range cases {
		if got := blockedResolvedIP(net.ParseIP(ipStr)); got != want {
			t.Errorf("blockedResolvedIP(%s) = %v, want %v", ipStr, got, want)
		}
	}
	if !blockedResolvedIP(nil) {
		t.Error("blockedResolvedIP(nil) = false, want true (fail closed)")
	}
}

// #251: the default client's transport refuses to connect to an internal
// address even when a redirect points there.
func TestGuardedTransport_RefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("internal"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: guardedTransport(), Timeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)

	if _, err := client.Do(req); err == nil {
		t.Fatal("expected the guarded transport to refuse a loopback connection")
	} else {
		var blocked *blockedAddressError
		if !errors.As(err, &blocked) {
			t.Fatalf("expected *blockedAddressError, got %v", err)
		}
	}
}

// #251: the redirect chain is capped.
func TestCapRedirects(t *testing.T) {
	if err := capRedirects(nil, make([]*http.Request, maxRedirectHops-1)); err != nil {
		t.Errorf("chain of %d = %v, want nil", maxRedirectHops-1, err)
	}
	if err := capRedirects(nil, make([]*http.Request, maxRedirectHops)); err == nil {
		t.Errorf("chain of %d = nil, want an error", maxRedirectHops)
	}
}
