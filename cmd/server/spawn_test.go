//go:build linux || windows

package main

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// uninitDataDirStat is a stat fake for a data directory that has not been
// initialized (no PG_VERSION) but exists with the 0700 mode initdb sets.
func uninitDataDirStat(name string) (os.FileInfo, error) {
	if filepath.Base(name) == "PG_VERSION" {
		return nil, os.ErrNotExist
	}
	return fakeDirInfo{}, nil
}

type fakeDirInfo struct{ os.FileInfo }

func (fakeDirInfo) Mode() os.FileMode { return fs.ModeDir | 0o700 }

// Tests that when the spawn step's command runner fails unconditionally,
// spawnPostgresOnce fails cleanly without internal retries (retry logic
// belongs to waitForPostgres).
func TestSpawnPostgresOnce_CommandRunnerFailureFailsCleanly(t *testing.T) {
	runCalls := 0
	failing := errors.New("initdb: could not create directory: permission denied")

	deps := supervisorDeps{
		userConfigDir: func() (string, error) { return t.TempDir(), nil },
		lookup:        func(file string) (string, error) { return "/usr/bin/" + file, nil },
		stat:          uninitDataDirStat,
		runCommand: func(context.Context, string, ...string) error {
			runCalls++
			return failing
		},
		selectPort: func() (int, error) {
			t.Fatal("selectPort must not be called when data directory init fails")
			return 0, nil
		},
		spawnChild: func(*exec.Cmd) error {
			t.Fatal("spawnChild must not be called when data directory init fails")
			return nil
		},
		dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("dial must not be called when data directory init fails")
			return nil, nil
		},
		attemptTimeout: 5 * time.Second,
	}

	state := &spawnState{}
	err := spawnPostgresOnce(context.Background(), state, deps)
	if !errors.Is(err, failing) {
		t.Fatalf("spawnPostgresOnce() error = %v, want the command runner's failure propagated", err)
	}
	if runCalls != 1 {
		t.Fatalf("command runner invoked %d times, want exactly 1 (no automatic retry inside spawnPostgresOnce itself)", runCalls)
	}
}

// A second call within the same spawnState must not re-run initdb or re-spawn;
// only the connectivity check repeats.
func TestSpawnPostgresOnce_SecondCallReusesStateAndOnlyWaitsForConnection(t *testing.T) {
	runCalls, spawnCalls, dialCalls := 0, 0, 0

	deps := supervisorDeps{
		userConfigDir: func() (string, error) { return t.TempDir(), nil },
		lookup:        func(file string) (string, error) { return "/usr/bin/" + file, nil },
		stat:          uninitDataDirStat,
		runCommand: func(context.Context, string, ...string) error {
			runCalls++
			return nil
		},
		selectPort: func() (int, error) { return 54329, nil },
		spawnChild: func(*exec.Cmd) error {
			spawnCalls++
			return nil
		},
		dial: func(context.Context, string, string) (net.Conn, error) {
			dialCalls++
			return fakeConn{}, nil
		},
		attemptTimeout: 5 * time.Second,
	}

	state := &spawnState{}
	if err := spawnPostgresOnce(context.Background(), state, deps); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if runCalls != 1 || spawnCalls != 1 {
		t.Fatalf("after first call: runCalls=%d spawnCalls=%d, want 1 and 1", runCalls, spawnCalls)
	}

	if err := spawnPostgresOnce(context.Background(), state, deps); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if runCalls != 1 || spawnCalls != 1 {
		t.Fatalf("after second call: runCalls=%d spawnCalls=%d, want still 1 and 1 (no re-init, no re-spawn)", runCalls, spawnCalls)
	}
	if dialCalls != 2 {
		t.Fatalf("dial invoked %d times across both calls, want 2 (connectivity re-checked every call)", dialCalls)
	}
}

// Without this, the app never becomes usable after a successful spawn:
// SelectStartupPath returning nil only means the raw TCP dial succeeded
// (spawnPostgresOnce's own readiness check) — nothing else in run()'s
// startup sequence (runMigrations, newPool) has any way to find the
// instance it just started unless cfg.DatabaseURL now points at it.
func TestObtainPostgres_SetsDatabaseURLOnSuccessfulSpawn(t *testing.T) {
	spawnDeps := supervisorDeps{
		userConfigDir:  func() (string, error) { return t.TempDir(), nil },
		lookup:         func(file string) (string, error) { return "/usr/bin/" + file, nil },
		stat:           uninitDataDirStat,
		runCommand:     func(context.Context, string, ...string) error { return nil },
		selectPort:     func() (int, error) { return 54329, nil },
		spawnChild:     func(*exec.Cmd) error { return nil },
		dial:           func(context.Context, string, string) (net.Conn, error) { return fakeConn{}, nil },
		attemptTimeout: 5 * time.Second,
	}
	spawn, state := newSpawnPostgres(spawnDeps)

	cfg := &config.Config{}
	err := obtainPostgresWithSpawn(context.Background(), cfg, spawn, state, connectPostgres(cfg))
	if err != nil {
		t.Fatalf("obtainPostgresWithSpawn: %v", err)
	}

	got := cfg.DatabaseURL.Reveal()
	want := "postgres://" + supervisor.BundledSuperuser + "@127.0.0.1:54329/postgres?sslmode=disable"
	if got != want {
		t.Fatalf("cfg.DatabaseURL = %q, want %q", got, want)
	}
}

// A caller that already configured DATABASE_URL (the Docker/container
// target, or a desktop user's own "Advanced" override — not exercised by
// any real flow yet, but SelectStartupPath's own contract) must never
// have it silently overwritten by a spawn that never even ran.
func TestObtainPostgres_LeavesAConfiguredDatabaseURLAlone(t *testing.T) {
	spawn, state := newSpawnPostgres(supervisorDeps{
		spawnChild: func(*exec.Cmd) error {
			t.Fatal("spawn must not run when DatabaseURL is already configured")
			return nil
		},
	})

	cfg := &config.Config{DatabaseURL: "postgres://elsewhere/db"}
	connect := func(context.Context) error { return nil } // stands in for connectPostgres against a real DB
	err := obtainPostgresWithSpawn(context.Background(), cfg, spawn, state, connect)
	if err != nil {
		t.Fatalf("obtainPostgresWithSpawn: %v", err)
	}
	if cfg.DatabaseURL.Reveal() != "postgres://elsewhere/db" {
		t.Fatalf("cfg.DatabaseURL = %q, want unchanged", cfg.DatabaseURL.Reveal())
	}
}

// Tests spawn failure driven through run(), verifying that supervisor
// errors cleanly propagate through startup error handling.
func TestRun_ProductionSpawnFailure(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.postgresMaxAttempts = 1

	runCalls := 0
	failing := errors.New("initdb: could not create directory: permission denied")
	spawnDeps := supervisorDeps{
		userConfigDir: func() (string, error) { return t.TempDir(), nil },
		lookup:        func(file string) (string, error) { return "/usr/bin/" + file, nil },
		stat:          uninitDataDirStat,
		runCommand: func(context.Context, string, ...string) error {
			runCalls++
			return failing
		},
		selectPort: func() (int, error) {
			t.Fatal("selectPort must not be called when data directory init fails")
			return 0, nil
		},
		spawnChild: func(*exec.Cmd) error {
			t.Fatal("spawnChild must not be called when data directory init fails")
			return nil
		},
		dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("dial must not be called when data directory init fails")
			return nil, nil
		},
		attemptTimeout: 5 * time.Second,
	}
	spawn, state := newSpawnPostgres(spawnDeps)
	deps.obtainPostgres = func(ctx context.Context, cfg *config.Config) error {
		return obtainPostgresWithSpawn(ctx, cfg, spawn, state, connectPostgres(cfg))
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on spawn failure")
	}
	if !spy.Contains("postgres") {
		t.Fatal("no log line names the failing step (postgres)")
	}
	if runCalls != 1 {
		t.Fatalf("command runner invoked %d times, want exactly 1", runCalls)
	}
}

// Self-review finding: without a per-attempt timeout, a Postgres that
// starts but never becomes ready would block a single spawnPostgresOnce
// call (and the connectivity-wait loop inside it) forever, since the
// outer ctx (run()'s own lifetime) has no deadline of its own — turning
// waitForPostgres's bounded retry budget into an unbounded one for this
// one attempt. Proves the bound is real, mirroring
// TestConnectPostgresWithTimeout_BoundsABlackHoleHost's own pattern: a
// dial fake that always fails must still return within the configured
// attemptTimeout, not hang.
func TestSpawnPostgresOnce_BoundsAnAttemptThatNeverBecomesReady(t *testing.T) {
	deps := supervisorDeps{
		userConfigDir: func() (string, error) { return t.TempDir(), nil },
		lookup:        func(file string) (string, error) { return "/usr/bin/" + file, nil },
		stat:          uninitDataDirStat,
		runCommand:    func(context.Context, string, ...string) error { return nil },
		selectPort:    func() (int, error) { return 54329, nil },
		spawnChild:    func(*exec.Cmd) error { return nil },
		dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		},
		attemptTimeout: 100 * time.Millisecond,
	}

	state := &spawnState{}
	start := time.Now()
	err := spawnPostgresOnce(context.Background(), state, deps)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("spawnPostgresOnce() error = nil, want a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("spawnPostgresOnce took %v, want it bounded near the 100ms attemptTimeout, not left hanging", elapsed)
	}
}

type fakeConn struct{ net.Conn }

func (fakeConn) Close() error { return nil }
