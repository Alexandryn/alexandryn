package postgres

import (
	"context"
	"strconv"
)

// SelectStartupPath runs spawn if databaseURL is empty — the Electron-
// hosted target's production path, and the container-hosted target has
// no alternative to fall back to at all — or connect if databaseURL is
// present — the Electron target's dev/CI/test override, and the
// container-hosted target's normal production path (ADR 0015). Exactly
// one of the two ever runs (backend-persistence.md FR-5).
func SelectStartupPath(ctx context.Context, databaseURL string, spawn, connect func(ctx context.Context) error) error {
	if databaseURL == "" {
		return spawn(ctx)
	}
	return connect(ctx)
}

// PostgresArgs builds the argument list cmd/server passes to the
// postgres binary directly (Linux/Windows), or that cmd/pg-supervisor
// passes to it on its own behalf (macOS) — the portable subset
// backend-persistence.md FR-8 says MAY be shared between both spawn
// paths, since both need to produce the same arguments. port binds the
// instance to a specific, OS-assigned port (architecture-persistence.md
// FR-2, T25-D2's own selection mechanism) rather than Postgres's own
// compiled-in default; socket-directory decisions still belong to a
// later, more detailed pass, not invented in this task.
func PostgresArgs(dataDir string, port int) []string {
	return []string{
		"-D", dataDir,
		"-p", strconv.Itoa(port),
		// Bind only to loopback regardless of the binary's compiled-in
		// listen_addresses default — a bundled instance must never be
		// reachable from another machine (constitution §6, audit 0016
		// #264). Broader exposure is the Go server's job, behind
		// authentication.
		"-c", "listen_addresses=127.0.0.1",
		// Restrict the Unix socket to the owner so another local account
		// cannot connect to the instance without a credential.
		"-c", "unix_socket_permissions=0700",
	}
}
