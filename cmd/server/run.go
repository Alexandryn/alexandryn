package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// pgPool is the minimal interface run's step 6 needs from whatever
// internal/persistence/postgres.NewPool returns: transporthttp.Pinger for
// /readyz, plus Close for the graceful-shutdown sequence (FR-6). A real
// *pgxpool.Pool already implements both natively; tests use a fake.
type pgPool interface {
	transporthttp.Pinger
	Close()
}

// shutdownableServer is the minimal interface run needs from whatever
// serves HTTP: Serve to start (step 4), Shutdown to stop cleanly (FR-4). A
// real *http.Server already implements both natively; tests use a fake
// that never opens a real socket.
type shutdownableServer interface {
	Serve(l net.Listener) error
	Shutdown(ctx context.Context) error
}

// clock is the one method run needs from "now" — FR-5's grace-period
// deadline is computed from it instead of time.Now() directly, so a test
// can fix it and assert the exact deadline without any real waiting
// (go-backend-conventions: inject Clock, never call time.Now() inline).
// testutil.FakeClock satisfies this structurally.
type clock interface {
	Now() time.Time
}

// realClock is production's clock: the real wall clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// runDeps carries every one of run's constructor dependencies as fields,
// never package-level state (backend-service-lifecycle.md FR-2). main
// populates it with the real config/logging/transport/persistence
// packages; tests substitute fakes that record call order.
type runDeps struct {
	loadConfig func() (*config.Config, error)
	newLogger  func(cfg *config.Config) *slog.Logger
	newRouter  func(cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef) http.Handler
	listen     func(network, address string) (net.Listener, error)
	newServer  func(cfg *config.Config, handler http.Handler) shutdownableServer
	clock      clock

	// obtainPostgres makes one attempt at FR-1 step 5's "obtain a
	// reachable PostgreSQL" (spawn-or-connect, chosen by DATABASE_URL's
	// presence) — retried up to postgresMaxAttempts times with
	// postgresBackoff between attempts, the one FR-3 names as the sole
	// exception to "every other step fails once."
	obtainPostgres      func(ctx context.Context, cfg *config.Config) error
	postgresMaxAttempts int
	postgresBackoff     time.Duration
	sleep               func(d time.Duration)

	// runMigrations is a single-attempt, ordinary FR-3 failure — distinct
	// from obtainPostgres's bounded retry (Failure modes table).
	runMigrations func(ctx context.Context, cfg *config.Config) error

	// newPool constructs the connection pool once PostgreSQL is reachable
	// and migrated (FR-1 step 6); its result populates the pool reference
	// step 3 already wired into the router.
	newPool func(ctx context.Context, cfg *config.Config) (pgPool, error)

	stderr io.Writer
}

// waitForPostgres calls obtain up to maxAttempts times, sleeping backoff
// between failed attempts (never after the last one), and returns nil on
// the first success or the last attempt's error, wrapped, once the budget
// is exhausted — FR-3's bounded-retry carve-out for "wait for Postgres to
// become reachable," the only step allowed to retry at all.
func waitForPostgres(ctx context.Context, cfg *config.Config, obtain func(context.Context, *config.Config) error, maxAttempts int, backoff time.Duration, sleep func(time.Duration), logger *slog.Logger) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = obtain(ctx, cfg)
		if lastErr == nil {
			return nil
		}
		if attempt < maxAttempts {
			logger.Info("waiting for PostgreSQL to become reachable", "attempt", attempt, "maxAttempts", maxAttempts)
			sleep(backoff)
		}
	}
	return fmt.Errorf("postgresql did not become reachable after %d attempts: %w", maxAttempts, lastErr)
}

// run executes backend-service-lifecycle.md FR-1's startup sequence:
// load and validate config; construct the logger; construct the router
// and middleware chain with an atomically-held, still-empty pool
// reference wired to /healthz and /readyz (FR-7); bind the listener and
// start serving (the process is now "alive"); obtain a reachable
// PostgreSQL with FR-3's bounded retry, run migrations, construct the
// pool and populate the reference; then report ready. On ctx's
// cancellation (a shutdown signal), it stops accepting new connections
// and lets in-flight requests finish within the configured grace period
// before closing the pool (FR-4/5/6).
//
// Each step logs a line on success at info level (FR-1's observability
// requirement) once the logger exists; config failure — the only step
// that can fail before the logger is constructed — is reported to stderr
// instead, since there is no logger yet to report it through.
func run(ctx context.Context, deps runDeps) int {
	cfg, err := deps.loadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(deps.stderr, "startup failed at step \"config\": %v\n", err)
		return 1
	}

	logger := deps.newLogger(cfg)
	logger.Info("startup step completed", "step", "config")
	logger.Info("startup step completed", "step", "logger")

	poolRef := &transporthttp.PoolRef{}
	router := deps.newRouter(cfg, logger, poolRef)
	logger.Info("startup step completed", "step", "router")

	listener, err := deps.listen("tcp", cfg.BindAddress)
	if err != nil {
		logger.Error("startup failed", "step", "listen", "error", err.Error())
		return 1
	}
	logger.Info("startup step completed", "step", "listen", "address", listener.Addr().String())

	srv := deps.newServer(cfg, router)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()

	if err := waitForPostgres(ctx, cfg, deps.obtainPostgres, deps.postgresMaxAttempts, deps.postgresBackoff, deps.sleep, logger); err != nil {
		msg := err.Error()
		if cfg.DatabaseURL != "" {
			// FR-3's security amendment (review 0028): a pgx connection
			// or DSN-parse error can embed DATABASE_URL itself — the
			// same leak backend-http-transport.md FR-5 already redacts
			// for /readyz's response body, applied here to this log
			// line instead. The generic message only applies when a
			// DATABASE_URL is actually in play; the spawn path's own
			// errors carry no connection string to leak.
			msg = "could not connect to the configured database"
		}
		logger.Error("startup failed", "step", "postgres", "error", msg)
		return 1
	}
	logger.Info("startup step completed", "step", "postgres")

	if err := deps.runMigrations(ctx, cfg); err != nil {
		logger.Error("startup failed", "step", "migrate", "error", err.Error())
		return 1
	}
	logger.Info("startup step completed", "step", "migrate")

	pool, err := deps.newPool(ctx, cfg)
	if err != nil {
		msg := err.Error()
		if cfg.DatabaseURL != "" {
			// Same leak as the postgres step above: pgxpool.ParseConfig's
			// own error embeds the connection string (pgx redacts only
			// the password, not host/user/dbname) when DATABASE_URL is
			// malformed. Never surface a driver/parser error's own text
			// here when a DATABASE_URL is in play.
			msg = "could not construct the connection pool for the configured database"
		}
		logger.Error("startup failed", "step", "pool", "error", msg)
		return 1
	}
	poolRef.Set(pool)
	logger.Info("startup step completed", "step", "pool")

	// TODO(D1): repository construction is stubbed empty here —
	// phase 02's domain aggregates and repository interfaces
	// (backend-persistence.md FR-2) aren't buildable Go yet
	// (.claude/roadmap/02-domain/README.md). Checkpoint G
	// (tasks/todo.md) re-checks phase 02's status before T24 wires
	// real repositories in here.

	logger.Info("ready")

	select {
	case <-ctx.Done():
		// FR-4: stop accepting new connections immediately and let
		// in-flight requests finish within the configured grace period
		// (FR-5) — Shutdown does both. FR-6: the pool closes only after
		// Shutdown returns, whether it completed cleanly or the grace
		// period expired, never before and never concurrently.
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithDeadline(context.Background(), deps.clock.Now().Add(cfg.ShutdownGracePeriod))
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			// A DeadlineExceeded here means the grace period expired
			// with requests still in flight — expected under load
			// (Failure modes table), not an error to report; any other
			// error is unexpected and worth a line, but neither stops
			// the pool from closing below.
			logger.Error("shutdown did not complete cleanly", "error", err.Error())
		}
		pool.Close()
		logger.Info("shutdown complete")
		return 0
	case err := <-serveErr:
		// The server stopped on its own, not via a shutdown signal — no
		// Shutdown was called, but FR-6's "close the pool before the
		// process exits" applies regardless of why the process is
		// exiting.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err.Error())
			pool.Close()
			return 1
		}
		pool.Close()
		return 0
	}
}
