//go:build spawn

package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// backend-test-harness.md FR-7's dedicated bundled-spawn suite:
// architecture-persistence.md FR-1/FR-5/FR-6/FR-8/FR-9 and
// backend-persistence.md FR-5/FR-6/FR-8 exercised against the real
// bundled-Postgres mechanism (E1-E5), not a service container — a
// separate build tag and (per FR-7) potentially a separate CI cadence
// from the fast per-PR integration suite, since it needs real
// postgres/initdb binaries and a real spawned process, not a Postgres
// service container GitHub Actions already provides for the tagged
// `integration` suite.
//
// This authoring environment has neither binary on PATH (this session's
// local Supabase stack runs Postgres inside Docker, not exposed as a
// host binary) — every test below skips cleanly with a named reason
// rather than failing cryptically or silently passing (T25-D4). None of
// this suite has been executed in this session; it is written and
// reviewed, not verified, exactly like E3's Windows code carries the
// same honest caveat. First real execution happens wherever CI (T27's
// own job to wire this suite in) or a developer machine actually has
// postgres/initdb installed.

func requirePostgresBinaries(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("postgres"); err != nil {
		t.Skip("postgres binary not found on PATH — the spawn suite needs the real bundled-Postgres mechanism, not a service container (backend-test-harness.md FR-7)")
	}
	if _, err := exec.LookPath("initdb"); err != nil {
		t.Skip("initdb binary not found on PATH — the spawn suite needs the real bundled-Postgres mechanism, not a service container (backend-test-harness.md FR-7)")
	}
}

// spawnE2EConfig is integrationConfig's own shape, minus a DatabaseURL —
// its absence is exactly what makes cmd/server take FR-5's spawn branch
// instead of connecting to a pre-existing service container.
func spawnE2EConfig() *config.Config {
	return &config.Config{
		LogLevel:            "error",
		BindAddress:         "127.0.0.1:0",
		HTTPMaxBodyBytes:    1 << 20,
		HTTPReadTimeout:     5 * time.Second,
		HTTPWriteTimeout:    5 * time.Second,
		HTTPIdleTimeout:     5 * time.Second,
		ShutdownGracePeriod: 2 * time.Second,
		DBPoolMaxConns:      5,
	}
}

// runSpawnColdStart drives the full FR-1/FR-5/FR-6 production sequence
// through run() itself — the real newObtainPostgres(), not a fake —
// with configDir set via XDG_CONFIG_HOME (T25-D3's own data directory
// derivation, os.UserConfigDir) so this test controls exactly where the
// spawned instance's data directory lands, and asserts /readyz reaches
// 200 within a generous bound before shutting the process down cleanly.
func runSpawnColdStart(t *testing.T, configDir string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cfg := spawnE2EConfig()
	addrCh := make(chan string, 1)

	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           quietLogger,
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      newObtainPostgres(),
		postgresMaxAttempts: 5,
		postgresBackoff:     500 * time.Millisecond,
		sleep:               sleepOrDone,
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			return postgres.Migrate(ctx, cfg.DatabaseURL.Reveal())
		},
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
			pool, err := postgres.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBPoolMaxConns)
			if err != nil {
				return nil, nil, err
			}
			return pool, newRepositories(pool), nil
		},
		stderr: io.Discard,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exitCh := make(chan int, 1)
	go func() { exitCh <- run(ctx, deps) }()

	addr := waitForAddr(t, addrCh, 5*time.Second)
	waitForStatus(t, "http://"+addr+"/readyz", http.StatusOK, 60*time.Second)

	cancel()
	if code := waitForExit(t, exitCh, 10*time.Second); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

// architecture-persistence.md's own named "fresh install" walkthrough:
// no data directory exists, the Go server initializes one, spawns
// PostgreSQL for real, runs real migrations, reaches Ready.
func TestSpawn_FreshDataDirectoryReachesReady(t *testing.T) {
	requirePostgresBinaries(t)
	runSpawnColdStart(t, t.TempDir())
}

// The spec's own named "second walkthrough": an existing,
// already-initialized data directory reaches Ready without repeating
// initdb — proven here against a real data directory a real initdb
// already populated, driven through the real run() sequence, not a
// direct EnsureDataDir unit call the way E5's own retry-safety test
// already covered in-memory.
func TestSpawn_ExistingDataDirectoryReachesReady(t *testing.T) {
	requirePostgresBinaries(t)

	configDir := t.TempDir()
	dataDir := filepath.Join(configDir, "alexandryn", "pgdata")

	bins, err := supervisor.LocateBinaries(exec.LookPath)
	if err != nil {
		t.Fatalf("LocateBinaries: %v", err)
	}
	runCommand := func(ctx context.Context, name string, args ...string) error {
		return exec.CommandContext(ctx, name, args...).Run()
	}
	if err := supervisor.EnsureDataDir(context.Background(), os.Stat, runCommand, bins.InitDB, dataDir); err != nil {
		t.Fatalf("pre-initializing the data directory: %v", err)
	}

	runSpawnColdStart(t, configDir)
}

// architecture-persistence.md's own named "hostile walkthrough": a data
// directory deliberately corrupted between runs (simulating a crash
// mid-write) must reach Failed, never attempt automatic
// deletion/reinitialization (FR-7). pg_control is Postgres's own
// first-checked control file — truncating it is a standard, realistic
// corruption simulation, distinct from removing PG_VERSION entirely
// (which would make EnsureDataDir treat the directory as merely
// uninitialized, not corrupted — a different, already-covered case).
func TestSpawn_CorruptedDataDirectoryFailsCleanly(t *testing.T) {
	requirePostgresBinaries(t)

	configDir := t.TempDir()
	dataDir := filepath.Join(configDir, "alexandryn", "pgdata")

	bins, err := supervisor.LocateBinaries(exec.LookPath)
	if err != nil {
		t.Fatalf("LocateBinaries: %v", err)
	}
	runCommand := func(ctx context.Context, name string, args ...string) error {
		return exec.CommandContext(ctx, name, args...).Run()
	}
	if err := supervisor.EnsureDataDir(context.Background(), os.Stat, runCommand, bins.InitDB, dataDir); err != nil {
		t.Fatalf("pre-initializing the data directory: %v", err)
	}

	controlFile := filepath.Join(dataDir, "global", "pg_control")
	if err := os.Truncate(controlFile, 0); err != nil {
		t.Fatalf("corrupting pg_control: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", configDir)
	cfg := spawnE2EConfig()
	addrCh := make(chan string, 1)

	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           quietLogger,
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      newObtainPostgres(),
		postgresMaxAttempts: 1,
		postgresBackoff:     0,
		sleep:               sleepOrDone,
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			return postgres.Migrate(ctx, cfg.DatabaseURL.Reveal())
		},
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
			pool, err := postgres.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBPoolMaxConns)
			if err != nil {
				return nil, nil, err
			}
			return pool, newRepositories(pool), nil
		},
		stderr: io.Discard,
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero — a corrupted data directory must fail startup, never silently succeed")
	}

	// FR-7's prohibition: no automatic deletion/reinitialization. A
	// PG_VERSION file created by the pre-initialization above must still
	// be exactly what it was — this process must not have touched the
	// data directory trying to "recover" it.
	if _, err := os.Stat(filepath.Join(dataDir, "PG_VERSION")); err != nil {
		t.Fatalf("PG_VERSION missing after a failed start — the data directory was modified: %v", err)
	}

	// The corruption itself must still be exactly as this test made it —
	// not "fixed" by any recovery attempt, and not made worse either
	// (e.g. deleted and left in a different broken state).
	info, err := os.Stat(controlFile)
	if err != nil {
		t.Fatalf("pg_control missing after a failed start — the data directory was modified: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("pg_control size = %d after a failed start, want still 0 (untouched, not repaired)", info.Size())
	}
}
