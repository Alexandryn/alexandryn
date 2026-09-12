package supervisor

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

// DialFunc matches the one shape this package needs to check whether
// something is listening on host:port yet — injected so tests can
// simulate "not ready yet, then ready" without a real network delay.
// Production passes a real net.Dialer's DialContext.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// WaitForConnection polls dial against host:port until it succeeds or
// ctx is done, sleeping interval between attempts — waiting for
// Postgres to accept connections as a distinct step after spawn, separate
// from spawn itself succeeding merely at starting the process.
func WaitForConnection(ctx context.Context, dial DialFunc, host string, port int, interval time.Duration) error {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	for {
		conn, err := dial(ctx, "tcp", address)
		if err == nil {
			return conn.Close()
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for PostgreSQL to accept connections at %s: %w", address, ctx.Err())
		case <-time.After(interval):
		}
	}
}
