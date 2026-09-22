package supervisor_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// fakeFileInfo is a minimal os.FileInfo carrying just a mode — the one
// field this package inspects.
type fakeFileInfo struct {
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return "" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

// statDir returns a stat fake: PG_VERSION missing (so initdb runs) and
// the data directory itself reporting dirMode.
func statDir(dataDir string, dirMode os.FileMode) supervisor.StatFunc {
	return func(name string) (os.FileInfo, error) {
		if name == dataDir {
			return fakeFileInfo{mode: fs.ModeDir | dirMode}, nil
		}
		return nil, os.ErrNotExist
	}
}

// Initialize the data directory if absent. "Absent" here means no PG_VERSION marker file.
func TestEnsureDataDir_RunsInitDBWhenNotInitialized(t *testing.T) {
	var ranWith []string
	run := func(_ context.Context, name string, args ...string) error {
		ranWith = append([]string{name}, args...)
		return nil
	}

	err := supervisor.EnsureDataDir(context.Background(), statDir("/data/pg", 0o700), run, "/usr/bin/initdb", "/data/pg")
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

// A fixed superuser name, not whatever OS user is running the desktop
// app, so the connection string spawn.go builds afterward does not have
// to guess it.
func TestEnsureDataDir_InitializesWithTheFixedSuperuserName(t *testing.T) {
	var ranWith []string
	run := func(_ context.Context, name string, args ...string) error {
		ranWith = append([]string{name}, args...)
		return nil
	}

	if err := supervisor.EnsureDataDir(context.Background(), statDir("/data/pg", 0o700), run, "/usr/bin/initdb", "/data/pg"); err != nil {
		t.Fatalf("EnsureDataDir: %v", err)
	}

	found := false
	for i, a := range ranWith {
		if a == "-U" && i+1 < len(ranWith) && ranWith[i+1] == supervisor.BundledSuperuser {
			found = true
		}
	}
	if !found {
		t.Fatalf("initdb args %v don't include -U %s", ranWith, supervisor.BundledSuperuser)
	}
}

// A directory that already has PG_VERSION is already initialized — a
// retried call must not re-run initdb against it.
func TestEnsureDataDir_SkipsInitDBWhenAlreadyInitialized(t *testing.T) {
	ranCount := 0
	run := func(context.Context, string, ...string) error {
		ranCount++
		return nil
	}
	stat := func(name string) (os.FileInfo, error) {
		if filepath.Base(name) == "PG_VERSION" {
			return fakeFileInfo{}, nil
		}
		if name == "/data/pg" {
			return fakeFileInfo{mode: fs.ModeDir | 0o700}, nil
		}
		return nil, os.ErrNotExist
	}

	err := supervisor.EnsureDataDir(context.Background(), stat, run, "/usr/bin/initdb", "/data/pg")
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

	err := supervisor.EnsureDataDir(context.Background(), statDir("/data/pg", 0o700), run, "/usr/bin/initdb", "/data/pg")
	if !errors.Is(err, failing) {
		t.Fatalf("EnsureDataDir() error = %v, want the initdb failure propagated", err)
	}
}

// Verifies that a data directory left group- or world-accessible is
// rejected, fail-closed, rather than handed to a Postgres instance.
func TestEnsureDataDir_RejectsLoosePermissions(t *testing.T) {
	run := func(context.Context, string, ...string) error { return nil }

	err := supervisor.EnsureDataDir(context.Background(), statDir("/data/pg", 0o755), run, "/usr/bin/initdb", "/data/pg")
	if err == nil {
		t.Fatal("EnsureDataDir accepted a 0755 data directory, want an error")
	}
}

// Verifies that a relative data directory is refused before any command runs.
func TestEnsureDataDir_RejectsRelativePath(t *testing.T) {
	ran := false
	run := func(context.Context, string, ...string) error { ran = true; return nil }

	err := supervisor.EnsureDataDir(context.Background(), statDir("relative/pg", 0o700), run, "/usr/bin/initdb", "relative/pg")
	if err == nil {
		t.Fatal("EnsureDataDir accepted a relative data directory path, want an error")
	}
	if ran {
		t.Fatal("initdb was run for a relative data directory path")
	}
}
