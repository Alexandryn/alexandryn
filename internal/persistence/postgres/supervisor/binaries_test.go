package supervisor_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// Binary location via an injected lookup function, ensuring test isolation
// from the host machine's PATH.
func TestLocateBinaries_ResolvesBothBinaries(t *testing.T) {
	lookup := func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	bins, err := supervisor.LocateBinaries(lookup)
	if err != nil {
		t.Fatalf("LocateBinaries: %v", err)
	}
	if bins.Postgres != "/usr/bin/postgres" {
		t.Fatalf("Postgres = %q, want /usr/bin/postgres", bins.Postgres)
	}
	if bins.InitDB != "/usr/bin/initdb" {
		t.Fatalf("InitDB = %q, want /usr/bin/initdb", bins.InitDB)
	}
}

// A specific, named error must identify which binary is missing,
// rather than exec.LookPath's bare error as the only surfaced detail.
func TestLocateBinaries_MissingPostgresNamesWhichBinary(t *testing.T) {
	lookupErr := errors.New("exec: \"postgres\": executable file not found in $PATH")
	lookup := func(file string) (string, error) {
		if file == "postgres" {
			return "", lookupErr
		}
		return "/usr/bin/" + file, nil
	}

	_, err := supervisor.LocateBinaries(lookup)
	if err == nil {
		t.Fatal("LocateBinaries() error = nil, want a named error for the missing postgres binary")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("error = %q, want it to name \"postgres\" specifically", err.Error())
	}
	if !errors.Is(err, lookupErr) {
		t.Fatalf("error = %v, want the underlying lookup error still reachable via errors.Is", err)
	}
}

// If PATH contains a relative or current-directory entry, exec.LookPath
// can resolve "postgres" to a relative path. Running that would execute a binary
// from an untrusted directory, so it must be rejected.
func TestLocateBinaries_RejectsNonAbsoluteResolution(t *testing.T) {
	lookup := func(file string) (string, error) {
		return "./" + file, nil
	}

	if _, err := supervisor.LocateBinaries(lookup); err == nil {
		t.Fatal("LocateBinaries accepted a relative binary path, want an error")
	}
}

func TestLocateBinaries_MissingInitDBNamesWhichBinary(t *testing.T) {
	lookupErr := errors.New("exec: \"initdb\": executable file not found in $PATH")
	lookup := func(file string) (string, error) {
		if file == "initdb" {
			return "", lookupErr
		}
		return "/usr/bin/" + file, nil
	}

	_, err := supervisor.LocateBinaries(lookup)
	if err == nil {
		t.Fatal("LocateBinaries() error = nil, want a named error for the missing initdb binary")
	}
	if !strings.Contains(err.Error(), "initdb") {
		t.Fatalf("error = %q, want it to name \"initdb\" specifically", err.Error())
	}
}
