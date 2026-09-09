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
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// uninitDataDirStat is a stat fake for a data directory that has not been
// initialized (no PG_VERSION) but exists with the 0700 mode initdb sets —
// the shape EnsureDataDir now checks after running initdb (audit 0016 #264).
func uninitDataDirStat(name string) (os.FileInfo, error) {
	if filepath.Base(name) == "PG_VERSION" {
		return nil, os.ErrNotExist
	}
	return fakeDirInfo{}, nil
}

type fakeDirInfo struct{ os.FileInfo }

func (fakeDirInfo) Mode() os.FileMode { return fs.ModeDir | 0o700 }

// backend-persistence.md's own required acceptance criterion
// (architecture-persistence.md FR-7's named failure classes: corrupted
// data directory, disk full): the spawn step's command runner is
// replaced with a fake that fails unconditionally. spawnPostgresOnce
// must fail cleanly and never retry internally — bounded retry belongs
// to waitForPostgres, one layer up, not this function — proven here by
// asserting the fake command runner (standing in for initdb) is invoked
// exactly once.
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

// T25-D3: a second call within the same spawnState must not re-run
// initdb or re-spawn — only the connectivity wait repeats.
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
		t.Fatalf("after second call: runCalls=%d spawnCalls=%d, want still 1 and 1 (T25-D3: no re-init, no re-spawn)", runCalls, spawnCalls)
	}
	if dialCalls != 2 {
		t.Fatalf("dial invoked %d times across both calls, want 2 (connectivity re-checked every call)", dialCalls)
	}
}

// The same proof, driven through the full run() sequence rather than
// spawnPostgresOnce directly — proves the real production wiring
// (newObtainPostgres -> postgres.SelectStartupPath -> newSpawnPostgres)
// composes correctly, with postgresMaxAttempts: 1 isolating the
// assertion the same way backend-service-lifecycle.md FR-3's other
// bounded-retry tests already do in run_test.go.
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
	spawn := newSpawnPostgres(spawnDeps)
	deps.obtainPostgres = func(ctx context.Context, cfg *config.Config) error {
		return postgres.SelectStartupPath(ctx, cfg.DatabaseURL.Reveal(), spawn, connectPostgres(cfg))
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
