//go:build integration

package main

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver goose needs
	"github.com/pressly/goose/v3"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// T20's Integration layer cases that need a real PostgreSQL: cold start
// to Ready, and the partial-migration-restart-detection path
// (backend-persistence.md FR-7's territory, exercised here through the
// full cmd/server startup sequence rather than the migration runner in
// isolation). Follows internal/persistence/postgres/migrate_integration_test.go's
// own TEST_DATABASE_URL/TestMain/resetSchema pattern.

func TestMain(m *testing.M) {
	os.Exit(testutil.IntegrationTestMain(os.LookupEnv, m.Run, os.Stderr))
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func resetSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("resetSchema: %v", err)
	}
}

func integrationConfig(dsn string) *config.Config {
	return &config.Config{
		LogLevel:            "error",
		BindAddress:         "127.0.0.1:0",
		HTTPMaxBodyBytes:    1 << 20,
		HTTPReadTimeout:     5 * time.Second,
		HTTPWriteTimeout:    5 * time.Second,
		HTTPIdleTimeout:     5 * time.Second,
		ShutdownGracePeriod: 2 * time.Second,
		DatabaseURL:         config.RedactedString(dsn),
		DBPoolMaxConns:      5,
	}
}

// Cold start to Ready against a real, empty database — phase 03's own
// exit criterion: migrations run, the pool connects, /readyz reaches 200
// over real HTTP, driven through the real production obtainPostgres
// (connect path, since DatabaseURL is set) and the real Migrate/NewPool.
func TestIntegration_ColdStartToReadyAgainstRealPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	resetSchema(t, testDB(t))

	cfg := integrationConfig(dsn)
	addrCh := make(chan string, 1)

	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           quietLogger,
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      obtainPostgres, // real production step 5, connect path (DatabaseURL is set)
		postgresMaxAttempts: 10,
		postgresBackoff:     200 * time.Millisecond,
		sleep:               sleepOrDone,
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			return postgres.Migrate(ctx, cfg.DatabaseURL.Reveal())
		},
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, error) {
			return postgres.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBPoolMaxConns)
		},
		stderr: io.Discard,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exitCh := make(chan int, 1)
	go func() { exitCh <- run(ctx, deps) }()

	addr := waitForAddr(t, addrCh, 5*time.Second)
	base := "http://" + addr

	waitForStatus(t, base+"/readyz", http.StatusOK, 15*time.Second)

	cancel()
	code := waitForExit(t, exitCh, 5*time.Second)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

// A data directory holding a migration applied partway: run() must exit
// non-zero at the migrate step, distinct from "unreachable," and never
// reach step 6 (pool construction) — backend-persistence.md FR-7's
// restart-detection behavior, proven through the full startup sequence
// rather than the migration runner in isolation.
func TestIntegration_PartialMigrationStopsBeforePool(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	resetSchema(t, testDB(t))

	dir := t.TempDir()
	writeMigration(t, dir, "00001_ok.sql", "CREATE TABLE cmdserver_partial_ok (id int);")
	writeMigration(t, dir, "00002_broken.sql", "CRATE TABLE cmdserver_partial_bad (id int);") // deliberate typo

	brokenMigrate := func(ctx context.Context, cfg *config.Config) error {
		up := func(ctx context.Context, db *sql.DB) error {
			goose.SetBaseFS(os.DirFS(dir))
			if err := goose.SetDialect("postgres"); err != nil {
				return err
			}
			return goose.UpContext(ctx, db, ".")
		}
		return postgres.RunMigrations(ctx, cfg.DatabaseURL.Reveal(), sql.Open, up)
	}

	cfg := integrationConfig(dsn)
	addrCh := make(chan string, 1)

	poolConstructed := false
	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           quietLogger,
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      obtainPostgres,
		postgresMaxAttempts: 10,
		postgresBackoff:     200 * time.Millisecond,
		sleep:               sleepOrDone,
		runMigrations:       brokenMigrate,
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, error) {
			poolConstructed = true
			return postgres.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBPoolMaxConns)
		},
		stderr: io.Discard,
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero — the broken migration must fail startup")
	}
	if poolConstructed {
		t.Fatal("newPool was called despite a failed migration — step 6 must never run")
	}
}

func writeMigration(t *testing.T, dir, name, upSQL string) {
	t.Helper()
	content := "-- +goose Up\n" + upSQL + "\n\n-- +goose Down\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeMigration(%q): %v", name, err)
	}
}
