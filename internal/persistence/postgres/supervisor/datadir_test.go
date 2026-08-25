package supervisor_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// architecture-persistence.md FR-1: initialize the data directory if
// absent. "Absent" here means no PG_VERSION marker file — Postgres's own
// signal that a directory is already an initialized data directory.
func TestEnsureDataDir_RunsInitDBWhenNotInitialized(t *testing.T) {
	var ranWith []string
	run := func(_ context.Context, name string, args ...string) error {
		ranWith = append([]string{name}, args...)
		return nil
	}
	statNotExist := func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	err := supervisor.EnsureDataDir(context.Background(), statNotExist, run, "/usr/bin/initdb", "/data/pg")
	if err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}
	if len(ranWith) == 0 {
		t.Fatal("initdb was never invoked for an uninitialized data directory")
	}
	if ranWith[0] != "/usr/bin/initdb" {
		t.Fatalf("ran %v, want initdb as the command", ranWith)
	}
	found := false
	for i, a := range ranWith {
		if a == "-D" && i+1 < len(ranWith) && ranWith[i+1] == "/data/pg" {
			found = true
		}
	}
	if !found {
		t.Fatalf("initdb args %v don't include -D /data/pg", ranWith)
	}
}

// A directory that already has PG_VERSION is already initialized — a
// retried call (waitForPostgres's own bounded retry, T25-D3) must not
// re-run initdb against it.
func TestEnsureDataDir_SkipsInitDBWhenAlreadyInitialized(t *testing.T) {
	ranCount := 0
	run := func(context.Context, string, ...string) error {
		ranCount++
		return nil
	}
	statExists := func(name string) (os.FileInfo, error) {
		if filepath.Base(name) == "PG_VERSION" {
			return fakeFileInfo{}, nil
		}
		return nil, os.ErrNotExist
	}

	err := supervisor.EnsureDataDir(context.Background(), statExists, run, "/usr/bin/initdb", "/data/pg")
	if err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}
	if ranCount != 0 {
		t.Fatalf("initdb was invoked %d times for an already-initialized data directory, want 0", ranCount)
	}
}

func TestEnsureDataDir_PropagatesInitDBFailure(t *testing.T) {
	failing := errors.New("initdb: could not create directory: permission denied")
	run := func(context.Context, string, ...string) error { return failing }
	statNotExist := func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	err := supervisor.EnsureDataDir(context.Background(), statNotExist, run, "/usr/bin/initdb", "/data/pg")
	if !errors.Is(err, failing) {
		t.Fatalf("EnsureDataDir() error = %v, want the initdb failure propagated", err)
	}
}

type fakeFileInfo struct{ os.FileInfo }
