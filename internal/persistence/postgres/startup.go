package postgres

import (
	"context"
	"strconv"
)

// SelectStartupPath runs spawn if databaseURL is empty — the Electron-
// hosted target's production path — or connect if databaseURL is
// present — the dev/CI/test override and container-hosted target's
// production path. Exactly one of the two ever runs.
func SelectStartupPath(ctx context.Context, databaseURL string, spawn, connect func(ctx context.Context) error) error {
	if databaseURL == "" {
		return spawn(ctx)
	}
	return connect(ctx)
}

// PostgresArgs builds the argument list passed to the postgres binary directly
// (Linux/Windows), or passed on its own behalf (macOS) — the portable subset
// shared between spawn paths to produce the same arguments. port binds the
// instance to a specific, OS-assigned port rather than Postgres's own
// compiled-in default.
func PostgresArgs(dataDir string, port int) []string {
	return []string{
		"-D", dataDir,
		"-p", strconv.Itoa(port),
		// Bind only to loopback regardless of the binary's compiled-in
		// listen_addresses default — a bundled instance must never be
		// reachable from another machine. Broader exposure is the Go
		// server's job, behind authentication.
		"-c", "listen_addresses=127.0.0.1",
		// Restrict the Unix socket to the owner so another local account
		// cannot connect to the instance without a credential.
		"-c", "unix_socket_permissions=0700",
	}
}
