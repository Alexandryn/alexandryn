//go:build integration

package main

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
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

// Integration tests that require a real PostgreSQL instance: cold start
// to Ready and partial migration restart detection, exercised through
// the full server startup sequence.

// TestMain provisions an isolated database for this test package.
func TestMain(m *testing.M) {
	os.Exit(testutil.IntegrationTestMain(os.LookupEnv, testutil.WithPackageDatabase("cmdserver", os.Getenv, os.Setenv, os.Stderr, m.Run), os.Stderr))
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

// Cold start to Ready against a real, empty database: migrations run,
// the pool connects, /readyz reaches 200 over HTTP, and repository
// implementations are constructed against the real connection pool.
func TestIntegration_ColdStartToReadyAgainstRealPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	resetSchema(t, testDB(t))

	cfg := integrationConfig(dsn)
	addrCh := make(chan string, 1)

	var capturedRepos *repositories
	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           quietLogger,
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      newObtainPostgres(), // real production step 5, connect path (DatabaseURL is set)
		postgresMaxAttempts: 10,
		postgresBackoff:     200 * time.Millisecond,
		sleep:               sleepOrDone,
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			return postgres.Migrate(ctx, cfg.DatabaseURL.Reveal())
		},
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
			pool, err := postgres.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBPoolMaxConns)
			if err != nil {
				return nil, nil, err
			}
			capturedRepos = newRepositories(pool)
			return pool, capturedRepos, nil
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

	assertRepositoriesConstructed(t, capturedRepos)
}

// A data directory holding a migration applied partway: run() must exit
// non-zero at the migrate step and never reach pool construction.
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
		obtainPostgres:      newObtainPostgres(),
		postgresMaxAttempts: 10,
		postgresBackoff:     200 * time.Millisecond,
		sleep:               sleepOrDone,
		runMigrations:       brokenMigrate,
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
			poolConstructed = true
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
		t.Fatal("exit code = 0, want non-zero — the broken migration must fail startup")
	}
	if poolConstructed {
		t.Fatal("newPool was called despite a failed migration — step 6 must never run")
	}
}

// assertRepositoriesConstructed verifies that every repository field
// set by newRepositories is populated.
func assertRepositoriesConstructed(t *testing.T, repos *repositories) {
	t.Helper()
	if repos == nil {
		t.Fatal("repositories were never constructed")
	}
	fields := map[string]any{
		"works":              repos.works,
		"authors":            repos.authors,
		"editions":           repos.editions,
		"libraryEntries":     repos.libraryEntries,
		"collections":        repos.collections,
		"sources":            repos.sources,
		"sourceOfferings":    repos.sourceOfferings,
		"readingProgress":    repos.readingProgress,
		"bookmarks":          repos.bookmarks,
		"highlights":         repos.highlights,
		"readingPreferences": repos.readingPreferences,
		"transactor":         repos.transactor,
		"metadataCache":      repos.metadataCache,
		"coverCache":         repos.coverCache,
		"users":              repos.users,
		"credentials":        repos.credentials,
		"refreshTokens":      repos.refreshTokens,
		"mfa":                repos.mfa,
		"passwordResets":     repos.passwordResets,
		"libraries":          repos.libraries,
		"libraryMemberships": repos.libraryMemberships,
		"libraryInvitations": repos.libraryInvitations,
		"pairedDevices":      repos.pairedDevices,
		"networkSettings":    repos.networkSettings,
		"enrolmentGrantJTIs": repos.enrolmentGrantJTIs,
		"networkSweep":       repos.networkSweep,
	}
	for name, field := range fields {
		if field == nil {
			t.Fatalf("repositories.%s is nil, want a real implementation", name)
		}
	}
}

// Migration success and failure logging: verifies that migration outcomes
// produce structured log lines distinguishable from connectivity failures.

// spyLogger is quietLogger's own shape, but backed by a
// testutil.SpyHandler so a test can inspect what was actually logged
// instead of discarding it.
func spyLogger(spy *testutil.SpyHandler) func(*config.Config) *slog.Logger {
	return func(*config.Config) *slog.Logger {
		return slog.New(spy)
	}
}

// hasLogRecord reports whether spy captured a record at level with
// message, carrying an attribute attrKey=attrValue — the concrete check
// this task's own distinguishability requirement needs: not just "some
// line mentions migrate," but a specific record, at the right level,
// naming the right step.
func hasLogRecord(spy *testutil.SpyHandler, level slog.Level, message, attrKey, attrValue string) bool {
	for _, r := range spy.Records() {
		if r.Level != level || r.Message != message {
			continue
		}
		found := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == attrKey && a.Value.String() == attrValue {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

func TestIntegration_MigrationSuccessLogsInfoNamingTheStep(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	resetSchema(t, testDB(t))

	cfg := integrationConfig(dsn)
	addrCh := make(chan string, 1)
	spy := testutil.NewSpyHandler()

	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           spyLogger(spy),
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      newObtainPostgres(),
		postgresMaxAttempts: 10,
		postgresBackoff:     200 * time.Millisecond,
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
	waitForStatus(t, "http://"+addr+"/readyz", http.StatusOK, 15*time.Second)

	cancel()
	if code := waitForExit(t, exitCh, 5*time.Second); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if !hasLogRecord(spy, slog.LevelInfo, "startup step completed", "step", "migrate") {
		t.Fatal("no info-level \"startup step completed\" record naming step=migrate")
	}
}

func TestIntegration_MigrationFailureLogsErrorDistinguishableFromPostgresStep(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	resetSchema(t, testDB(t))

	dir := t.TempDir()
	writeMigration(t, dir, "00001_ok.sql", "CREATE TABLE cmdserver_observability_ok (id int);")
	writeMigration(t, dir, "00002_broken.sql", "CRATE TABLE cmdserver_observability_bad (id int);") // deliberate typo

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
	spy := testutil.NewSpyHandler()

	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           spyLogger(spy),
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      newObtainPostgres(),
		postgresMaxAttempts: 10,
		postgresBackoff:     200 * time.Millisecond,
		sleep:               sleepOrDone,
		runMigrations:       brokenMigrate,
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
		t.Fatal("exit code = 0, want non-zero — the broken migration must fail startup")
	}
	if !hasLogRecord(spy, slog.LevelInfo, "startup step completed", "step", "postgres") {
		t.Fatal("no info-level \"startup step completed\" record naming step=postgres — the earlier connect step must have succeeded before migrate ran")
	}
	if !hasLogRecord(spy, slog.LevelError, "startup failed", "step", "migrate") {
		t.Fatal("no error-level \"startup failed\" record naming step=migrate")
	}
}

func writeMigration(t *testing.T, dir, name, upSQL string) {
	t.Helper()
	content := "-- +goose Up\n" + upSQL + "\n\n-- +goose Down\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeMigration(%q): %v", name, err)
	}
}

// poolStatsProvider must report live pool statistics to the diagnostics endpoint,
// not zeroed defaults.
func TestIntegration_PoolStatsProviderReportsLivePool(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	resetSchema(t, testDB(t))
	if err := postgres.Migrate(context.Background(), dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := postgres.NewPool(context.Background(), dsn, 5)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	// Hold a connection so AcquiredConns is provably non-zero.
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer conn.Release()

	stats := poolStatsProvider(pool)()
	if stats.MaxConns != 5 {
		t.Fatalf("MaxConns = %d, want 5", stats.MaxConns)
	}
	if stats.AcquiredConns < 1 {
		t.Fatalf("AcquiredConns = %d, want at least 1", stats.AcquiredConns)
	}
	if stats.TotalConns < stats.AcquiredConns {
		t.Fatalf("TotalConns %d < AcquiredConns %d", stats.TotalConns, stats.AcquiredConns)
	}
}
