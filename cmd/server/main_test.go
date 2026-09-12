package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// Correlation IDs must be randomly generated values never derived from
// request input. This tests the real generator directly, proving it
// produces distinct values across repeated invocations.
func TestNewCorrelationID_ProducesDistinctValues(t *testing.T) {
	const n = 1000
	seen := make(map[string]bool, n)

	for i := 0; i < n; i++ {
		id := newCorrelationID()
		if id == "" {
			t.Fatal("newCorrelationID() returned an empty string")
		}
		if seen[id] {
			t.Fatalf("newCorrelationID() produced a duplicate after %d calls: %q", i, id)
		}
		seen[id] = true
	}
}

// Without a per-attempt timeout, a host that accepts TCP connections but
// never completes the PostgreSQL startup handshake could block connection
// attempts indefinitely. This tests that connectPostgresWithTimeout returns
// within its configured timeout when a host accepts connections without responding.
func TestConnectPostgresWithTimeout_BoundsABlackHoleHost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start the test listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Accept the TCP handshake and then do nothing — never send
			// Postgres's own startup response, never close the
			// connection. This is the "black hole host" the finding
			// describes.
			_ = conn
		}
	}()

	cfg := &config.Config{
		DatabaseURL: config.RedactedString("postgres://user:pass@" + ln.Addr().String() + "/db"),
	}

	const timeout = 100 * time.Millisecond
	connect := connectPostgresWithTimeout(cfg, timeout)

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- connect(context.Background()) }()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("connect succeeded against a host that never completes the handshake, want an error")
		}
		// Generous slack over the configured timeout: this is proving
		// "bounded," not measuring exact scheduling precision.
		if elapsed > 2*time.Second {
			t.Fatalf("connect took %v to fail, want close to the configured %v timeout", elapsed, timeout)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connect never returned — the per-attempt timeout was not applied, the black-hole host blocked indefinitely")
	}
}
