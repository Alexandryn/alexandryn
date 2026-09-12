// Package supervisor provides binary location, data directory
// initialization, and port selection for managed PostgreSQL processes.
package supervisor

import (
	"fmt"
	"path/filepath"
)

// LookupFunc matches exec.LookPath's signature, allowing tests to inject
// mock resolution logic. Production callers pass exec.LookPath directly.
type LookupFunc func(file string) (string, error)

// Binaries holds the resolved locations of the two PostgreSQL binaries
// required by the supervisor.
type Binaries struct {
	Postgres string
	InitDB   string
}

// LocateBinaries resolves postgres and initdb via lookup, returning an
// error identifying which binary is missing or if a resolved path is unsafe.
func LocateBinaries(lookup LookupFunc) (Binaries, error) {
	postgresPath, err := lookup("postgres")
	if err != nil {
		return Binaries{}, fmt.Errorf("postgres binary not found on PATH: %w", err)
	}
	if !filepath.IsAbs(postgresPath) {
		return Binaries{}, fmt.Errorf("resolved postgres binary path %q is not absolute; refusing to run it (PATH may contain a relative or current-directory entry)", postgresPath)
	}
	initDBPath, err := lookup("initdb")
	if err != nil {
		return Binaries{}, fmt.Errorf("initdb binary not found on PATH: %w", err)
	}
	if !filepath.IsAbs(initDBPath) {
		return Binaries{}, fmt.Errorf("resolved initdb binary path %q is not absolute; refusing to run it", initDBPath)
	}
	return Binaries{Postgres: postgresPath, InitDB: initDBPath}, nil
}
