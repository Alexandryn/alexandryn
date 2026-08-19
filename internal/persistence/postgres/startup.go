package postgres

import "context"

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
// paths, since both need to produce the same arguments. Only the data
// directory is fixed here; port and socket-directory decisions belong to
// a later, more detailed pass, not invented in this task.
func PostgresArgs(dataDir string) []string {
	return []string{"-D", dataDir}
}
