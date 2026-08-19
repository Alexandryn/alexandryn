package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// backend-errors-and-logging.md FR-7: the correlation ID "MUST be a
// randomly generated value ... never derived from anything
// request-supplied." Every other test in this codebase exercises the
// logging/recovery middleware against testutil.FakeIDGenerator for
// determinism — this is the one test of the real generator itself,
// proving it actually produces distinct values rather than a fixed or
// predictable one.
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

// Checkpoint F's security review (MEDIUM): without a per-attempt timeout,
// a host that accepts the TCP connection but never completes Postgres's
// own startup handshake could block a single connectPostgres attempt
// indefinitely, turning the documented ~30-second retry budget
// (postgresReadyMaxAttempts * postgresReadyBackoff) into an unbounded
// one. This proves the bound is real: a real TCP listener accepts the
// connection and then sends nothing, ever — connectPostgresWithTimeout
// must still return within its configured timeout, not hang.
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
