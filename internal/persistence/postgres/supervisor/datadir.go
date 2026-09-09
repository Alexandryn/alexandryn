package supervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// CommandRunner executes an external command (initdb here; postgres
// itself in a later task) — injected so a test can fail it deliberately
// without running a real process, matching this codebase's existing
// Clock/FS/IDGenerator injection pattern.
type CommandRunner func(ctx context.Context, name string, args ...string) error

// StatFunc matches os.Stat's signature for the files this package
// checks — PG_VERSION (Postgres's own marker that a directory is already
// an initialized data directory) and the data directory itself (for its
// permission bits).
type StatFunc func(name string) (os.FileInfo, error)

// EnsureDataDir initializes dataDir via initDBPath if it isn't already a
// PostgreSQL data directory (architecture-persistence.md FR-1: "applies
// if the directory does not already exist") — a no-op when PG_VERSION is
// already present, so a retried call (waitForPostgres's own bounded
// retry, T25-D3) doesn't attempt to run initdb a second time against a
// directory a previous attempt already initialized.
//
// dataDir must be an absolute path (audit 0016 #264 — a value that
// traces to $XDG_CONFIG_HOME must not be able to start with a dash or be
// interpreted relative to the working directory), and after
// initialization the directory must be inaccessible to group and other
// (0700) — the mode initdb sets and PostgreSQL refuses to start without.
func EnsureDataDir(ctx context.Context, stat StatFunc, run CommandRunner, initDBPath, dataDir string) error {
	if !filepath.IsAbs(dataDir) {
		return fmt.Errorf("PostgreSQL data directory %q must be an absolute path", dataDir)
	}

	if _, err := stat(filepath.Join(dataDir, "PG_VERSION")); err == nil {
		return checkDataDirPerms(stat, dataDir)
	}

	if err := run(ctx, initDBPath, "-D", dataDir); err != nil {
		return err
	}
	return checkDataDirPerms(stat, dataDir)
}

// checkDataDirPerms fails closed if the data directory is readable or
// writable by group or other. PostgreSQL itself enforces 0700/0750; this
// catches a directory whose mode was loosened after a previous run
// before the instance is handed reading data (constitution §8).
func checkDataDirPerms(stat StatFunc, dataDir string) error {
	fi, err := stat(dataDir)
	if err != nil {
		return fmt.Errorf("checking PostgreSQL data directory permissions: %w", err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("PostgreSQL data directory %s has mode %04o; it must be inaccessible to group and other (0700)", dataDir, perm)
	}
	return nil
}
