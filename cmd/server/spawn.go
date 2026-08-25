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

// spawnState carries FR-1 step 5's spawn-once, wait-many state across
// waitForPostgres's own bounded retry within a single run() invocation
// (T25-D3, refined during implementation from
// tasks/plan-t25-persistence-e2e.md's original postmaster.pid-reading
// proposal: reading Postgres's own lock file from outside pg_ctl would
// need distinguishing a lock-conflict process exit from a genuine
// corruption exit, which this codebase's CommandRunner — an opaque
// error, no exit-code/stderr detail — can't do without real empirical
// testing against a real postgres binary, not available in this
// environment). In-memory state closed over by the same long-lived
// closure waitForPostgres calls repeatedly needs no such distinction:
// the first call resolves the data directory, initializes it if needed,
// picks a port, and spawns; every later call within the same run()
// invocation reuses that port and only re-checks connectivity, never
// re-runs initdb or re-spawns. A restart of the whole Alexandryn process
// is a different scenario, already covered by
// architecture-persistence.md's own named failure mode (a stale port
// bind failing cleanly on next start) — out of this plan's scope.
type spawnState struct {
	dataDir string
	started bool
	port    int
}

// supervisorDeps bundles every dependency spawnPostgresOnce needs,
// injected so a test can replace any one of them — most commonly just
// runCommand, for the production-spawn-failure proof — without a real
// postgres/initdb binary, a real filesystem, or a real network.
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

// productionSupervisorDeps is supervisorDeps' real implementation, every
// field a thin wrapper around a real OS interaction.
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

// newSpawnPostgres returns FR-1 step 5's spawn function for
// postgres.SelectStartupPath, closing over one spawnState for the
// lifetime of the returned closure — constructed once (FR-2), not
// package-level state.
func newSpawnPostgres(deps supervisorDeps) func(ctx context.Context) error {
	state := &spawnState{}
	return func(ctx context.Context) error {
		return spawnPostgresOnce(ctx, state, deps)
	}
}

// newObtainPostgres is FR-1 step 5's real Linux/Windows implementation:
// spawn a bundled instance when DATABASE_URL is absent
// (architecture-persistence.md FR-1/FR-8/FR-9), or connect directly when
// present (backend-persistence.md FR-5) — exactly one of the two, chosen
// by postgres.SelectStartupPath.
func newObtainPostgres() func(ctx context.Context, cfg *config.Config) error {
	spawn := newSpawnPostgres(productionSupervisorDeps())
	return func(ctx context.Context, cfg *config.Config) error {
		return postgres.SelectStartupPath(ctx, cfg.DatabaseURL.Reveal(), spawn, connectPostgres(cfg))
	}
}

// postgresSpawnAttemptTimeout bounds one spawnPostgresOnce call — the
// same "give each retry-loop attempt its own internal bound, since the
// outer ctx (run()'s own lifetime, canceled only on shutdown) has none"
// pattern connectPostgresWithTimeout already establishes for the connect
// path. Without this, a Postgres that starts but never becomes ready
// would block a single retry attempt (and WaitForConnection's own loop
// inside it) forever, never returning control to waitForPostgres's outer
// loop to log progress or exhaust its attempt budget. Generous: covers a
// from-scratch initdb plus PostgreSQL's own startup on a slow disk, not
// just steady-state connectivity.
const postgresSpawnAttemptTimeout = 30 * time.Second

// spawnPostgresOnce performs FR-1 step 5's spawn path: locate binaries,
// initialize the data directory if absent, pick a port, spawn with
// platform orphan-prevention, and wait for it to accept connections. A
// second call against the same state (state.started) skips straight to
// the connectivity wait (T25-D3).
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
