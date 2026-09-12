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

// Bundled PostgreSQL supervisor end-to-end test suite.
// Exercises data directory initialization and binary management against
// real postgres/initdb binaries when available on the host PATH.

func requirePostgresBinaries(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("postgres"); err != nil {
		t.Skip("postgres binary not found on PATH — the spawn suite needs real bundled-Postgres binaries")
	}
	if _, err := exec.LookPath("initdb"); err != nil {
		t.Skip("initdb binary not found on PATH — the spawn suite needs real bundled-Postgres binaries")
	}
}

// spawnE2EConfig returns a test configuration without DatabaseURL,
// directing the server to take the local spawn path.
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

// runSpawnColdStart drives the production startup sequence through run(),
// verifying that /readyz answers with 200 before cleanly shutting down.
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

// Tests cold start when no data directory exists: initializes the directory,
// spawns PostgreSQL, runs migrations, and reaches ready state.
func TestSpawn_FreshDataDirectoryReachesReady(t *testing.T) {
	requirePostgresBinaries(t)
	runSpawnColdStart(t, t.TempDir())
}

// Tests startup against an existing, pre-initialized data directory,
// verifying that initdb is not repeated and ready state is reached.
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

// Tests startup against a corrupted data directory (simulating a crash mid-write).
// Must fail cleanly without attempting automatic deletion or reinitialization.
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

	// No automatic deletion or reinitialization: a PG_VERSION file created by
	// pre-initialization must remain untouched after a failed start.
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
