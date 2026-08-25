package supervisor

import (
	"context"
	"os"
	"path/filepath"
)

// CommandRunner executes an external command (initdb here; postgres
// itself in a later task) — injected so a test can fail it deliberately
// without running a real process, matching this codebase's existing
// Clock/FS/IDGenerator injection pattern.
type CommandRunner func(ctx context.Context, name string, args ...string) error

// StatFunc matches os.Stat's signature for the one file this package
// checks — PG_VERSION, Postgres's own marker that a directory is already
// an initialized data directory.
type StatFunc func(name string) (os.FileInfo, error)

// EnsureDataDir initializes dataDir via initDBPath if it isn't already a
// PostgreSQL data directory (architecture-persistence.md FR-1: "applies
// if the directory does not already exist") — a no-op when PG_VERSION is
// already present, so a retried call (waitForPostgres's own bounded
// retry, T25-D3) doesn't attempt to run initdb a second time against a
// directory a previous attempt already initialized.
func EnsureDataDir(ctx context.Context, stat StatFunc, run CommandRunner, initDBPath, dataDir string) error {
	if _, err := stat(filepath.Join(dataDir, "PG_VERSION")); err == nil {
		return nil
	}

	return run(ctx, initDBPath, "-D", dataDir)
}
