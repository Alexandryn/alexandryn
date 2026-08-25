package supervisor

import (
	"fmt"
	"net"
)

// SelectPort binds a real 127.0.0.1:0 TCP listener, reads the
// OS-assigned port, and closes the listener immediately — the standard
// throwaway-probe technique for handing PostgreSQL a free port via its
// own -p flag, since Postgres has no analogous "assign me any free port"
// mode the way net.Listen's own :0 gives a Go net.Listener one. Real, not
// faked: an OS interaction with no faithful pure-logic substitute.
// Accepts a small window between this call returning and Postgres
// actually binding where another process could take the port (T25-D2, a
// recorded, accepted tradeoff — the same one embedded-postgres-style
// tools already accept).
func SelectPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("selecting a port for the spawned PostgreSQL instance: %w", err)
	}
	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("selecting a port for the spawned PostgreSQL instance: unexpected listener address type %T", ln.Addr())
	}
	return addr.Port, nil
}
