package supervisor_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// architecture-persistence.md's own State transitions: "wait for
// Postgres to accept connections" is a distinct step after spawn
// succeeds merely at starting the process. A fake dial that fails a
// fixed number of times, then succeeds, proves the wait phase keeps
// retrying rather than giving up on the first failed attempt.
func TestWaitForConnection_SucceedsOnceDialSucceeds(t *testing.T) {
	attempts := 0
	dial := func(context.Context, string, string) (net.Conn, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("connection refused")
		}
		return &fakeConn{}, nil
	}

	err := supervisor.WaitForConnection(context.Background(), dial, "127.0.0.1", 5432, time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForConnection: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("dial was attempted %d times, want 3", attempts)
	}
}

// A specific, named error when the context's own deadline is what ends
// the wait — never a bare "connection refused" as the only surfaced
// detail once the caller's bound is what actually gave up.
func TestWaitForConnection_TimesOutCleanlyWhenNeverReady(t *testing.T) {
	dial := func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("connection refused")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := supervisor.WaitForConnection(ctx, dial, "127.0.0.1", 5432, time.Millisecond)
	if err == nil {
		t.Fatal("WaitForConnection() error = nil, want a timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded reachable via errors.Is", err)
	}
}

type fakeConn struct{ net.Conn }

func (fakeConn) Close() error { return nil }
