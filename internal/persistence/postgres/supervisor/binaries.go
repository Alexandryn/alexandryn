// Package supervisor is the portable half of
// backend-persistence.md FR-5's spawn mechanism (T25,
// tasks/plan-t25-persistence-e2e.md) — binary location, data directory
// initialization, and port selection, shared by cmd/server's own
// Linux/Windows spawn path and (eventually) cmd/pg-supervisor's macOS
// path, per FR-8's "MAY be shared between both spawn paths" note. Package
// name kept as backend-persistence.md FR-5 itself names it — a "naming
// placeholder," not reconsidered here.
package supervisor

import "fmt"

// LookupFunc matches exec.LookPath's signature — injected (T25-D4) so
// tests don't depend on what's actually on this machine's PATH.
// Production passes exec.LookPath directly.
type LookupFunc func(file string) (string, error)

// Binaries holds the resolved locations of the two PostgreSQL binaries
// this package needs.
type Binaries struct {
	Postgres string
	InitDB   string
}

// LocateBinaries resolves postgres and initdb via lookup, returning a
// specific, named error identifying which binary is missing
// (backend-service-lifecycle.md FR-3) rather than surfacing lookup's own
// terse "executable file not found in $PATH" as the only detail.
func LocateBinaries(lookup LookupFunc) (Binaries, error) {
	postgresPath, err := lookup("postgres")
	if err != nil {
		return Binaries{}, fmt.Errorf("postgres binary not found on PATH: %w", err)
	}
	initDBPath, err := lookup("initdb")
	if err != nil {
		return Binaries{}, fmt.Errorf("initdb binary not found on PATH: %w", err)
	}
	return Binaries{Postgres: postgresPath, InitDB: initDBPath}, nil
}
