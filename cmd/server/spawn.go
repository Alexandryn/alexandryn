//go:build linux || windows

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// spawnState tracks supervisor state across retry attempts within a single
// run() invocation. The first call resolves the data directory, initializes it
// if needed, selects a port, and spawns the PostgreSQL process. Subsequent
// attempts within the same run() invocation reuse the existing port and verify
// connectivity without re-running initdb or re-spawning.
type spawnState struct {
	dataDir string
	started bool
	port    int
}

// supervisorDeps bundles dependencies needed by spawnPostgresOnce,
// injected to allow testing without requiring real PostgreSQL binaries.
type supervisorDeps struct {
	userConfigDir  func() (string, error)
	lookup         supervisor.LookupFunc
	stat           supervisor.StatFunc
	runCommand     supervisor.CommandRunner
	selectPort     func() (int, error)
	spawnChild     func(*exec.Cmd) error
	dial           supervisor.DialFunc
	attemptTimeout time.Duration
}

// productionSupervisorDeps returns supervisorDeps bound to real OS calls.
func productionSupervisorDeps() supervisorDeps {
	return supervisorDeps{
		userConfigDir: os.UserConfigDir,
		lookup:        exec.LookPath,
		stat:          os.Stat,
		runCommand: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		},
		selectPort:     supervisor.SelectPort,
		spawnChild:     supervisor.SpawnWithOrphanPrevention,
		dial:           (&net.Dialer{}).DialContext,
		attemptTimeout: postgresSpawnAttemptTimeout,
	}
}

// newSpawnPostgres returns a spawn function for postgres.SelectStartupPath,
// closing over one spawnState for the lifetime of the returned closure.
func newSpawnPostgres(deps supervisorDeps) func(ctx context.Context) error {
	state := &spawnState{}
	return func(ctx context.Context) error {
		return spawnPostgresOnce(ctx, state, deps)
	}
}

// newObtainPostgres provides the Linux/Windows startup implementation:
// spawn a bundled instance when DATABASE_URL is absent, or connect directly
// when present, selected by postgres.SelectStartupPath.
func newObtainPostgres() func(ctx context.Context, cfg *config.Config) error {
	spawn := newSpawnPostgres(productionSupervisorDeps())
	return func(ctx context.Context, cfg *config.Config) error {
		return postgres.SelectStartupPath(ctx, cfg.DatabaseURL.Reveal(), spawn, connectPostgres(cfg))
	}
}

// postgresSpawnAttemptTimeout bounds a single spawnPostgresOnce attempt.
// Covers initdb execution and initial PostgreSQL process startup.
const postgresSpawnAttemptTimeout = 30 * time.Second

// spawnPostgresOnce locates binaries, initializes the data directory if absent,
// selects a port, spawns PostgreSQL with orphan prevention, and waits for
// it to accept connections. Subsequent calls against an already-started state
// skip directly to the connectivity check.
func spawnPostgresOnce(ctx context.Context, state *spawnState, deps supervisorDeps) error {
	attemptCtx, cancel := context.WithTimeout(ctx, deps.attemptTimeout)
	defer cancel()

	if state.started {
		return supervisor.WaitForConnection(attemptCtx, deps.dial, "127.0.0.1", state.port, 100*time.Millisecond)
	}

	if state.dataDir == "" {
		configDir, err := deps.userConfigDir()
		if err != nil {
			return fmt.Errorf("determining the PostgreSQL data directory: %w", err)
		}
		state.dataDir = filepath.Join(configDir, "alexandryn", "pgdata")
	}

	bins, err := supervisor.LocateBinaries(deps.lookup)
	if err != nil {
		return err
	}
	if err := supervisor.EnsureDataDir(attemptCtx, deps.stat, deps.runCommand, bins.InitDB, state.dataDir); err != nil {
		return err
	}

	port, err := deps.selectPort()
	if err != nil {
		return err
	}
	cmd := exec.Command(bins.Postgres, postgres.PostgresArgs(state.dataDir, port)...)
	if err := deps.spawnChild(cmd); err != nil {
		return err
	}
	state.started = true
	state.port = port

	return supervisor.WaitForConnection(attemptCtx, deps.dial, "127.0.0.1", port, 100*time.Millisecond)
}
