package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
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
// serves HTTP: Serve to start (step 4), Shutdown to stop cleanly (FR-4),
// Close to force-close whatever Shutdown's grace period couldn't finish
// (Shutdown alone never touches active connections — it only waits for
// them; net/http's own docs say so). A real *http.Server already
// implements all three natively; tests use a fake that never opens a
// real socket.
type shutdownableServer interface {
	Serve(l net.Listener) error
	Shutdown(ctx context.Context) error
	Close() error
}

// jobRunner is the minimal surface run needs from the background job
// worker pool (backend-job-queue.md): start it once PostgreSQL is
// reachable (FR-1 step 6), stop it during graceful shutdown, between the
// HTTP server stopping and the shared pool closing
// (backend-service-lifecycle.md FR-6, amended for phase 09). *jobs.System
// implements it; tests use a fake that records call order.
type jobRunner interface {
	Start(ctx context.Context)
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
	// sleep must return early on ctx cancellation, not just after d — a
	// shutdown signal arriving mid-retry must interrupt the backoff wait,
	// not be absorbed by it (FR-4: a clean signal must never be treated
	// as an ordinary startup failure).
	sleep func(ctx context.Context, d time.Duration)

	// runMigrations is a single-attempt, ordinary FR-3 failure — distinct
	// from obtainPostgres's bounded retry (Failure modes table).
	runMigrations func(ctx context.Context, cfg *config.Config) error

	// newPool constructs the connection pool once PostgreSQL is reachable
	// and migrated (FR-1 step 6); its first result populates the pool
	// reference step 3 already wired into the router. Its second result
	// is every T24 repository implementation, constructed against that
	// same pool — the spec's own step 6 covers both in one step
	// ("construct pool... construct repositories"), so both are built by
	// one call rather than two separate hooks.
	newPool func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error)

	// newJobSystem constructs the background job worker pool against the
	// shared connection pool (FR-1 step 6). nil disables jobs — the
	// default for tests that don't exercise them. A non-nil hook
	// returning an error fails startup like any other step (FR-3).
	newJobSystem func(cfg *config.Config, logger *slog.Logger, pool pgPool) (jobRunner, error)

	// watchParent watches the Electron host parent process PID for termination (E25).
	watchParent func(pid int) error

	userConfigDir func() (string, error)

	stderr io.Writer
}

// waitForPostgres calls obtain up to maxAttempts times, sleeping backoff
// between failed attempts (never after the last one), and returns nil on
// the first success or the last attempt's error, wrapped, once the budget
// is exhausted — FR-3's bounded-retry carve-out for "wait for Postgres to
// become reachable," the only step allowed to retry at all.
//
// It checks ctx.Err() before every attempt and again after a failed one,
// returning it immediately rather than continuing to retry or sleep — a
// shutdown signal arriving mid-retry must stop the retry loop right away,
// not be treated as just another failed attempt (FR-4). Callers
// distinguish this case from an ordinary exhausted-budget failure by
// checking ctx.Err() on the returned error.
func waitForPostgres(ctx context.Context, cfg *config.Config, obtain func(context.Context, *config.Config) error, maxAttempts int, backoff time.Duration, sleep func(context.Context, time.Duration), logger *slog.Logger) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = obtain(ctx, cfg)
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt < maxAttempts {
			logger.Info("waiting for PostgreSQL to become reachable", "attempt", attempt, "maxAttempts", maxAttempts)
			sleep(ctx, backoff)
		}
	}
	return fmt.Errorf("postgresql did not become reachable after %d attempts: %w", maxAttempts, lastErr)
}

// gracefulShutdown runs FR-4/5/6's shutdown sequence: stop accepting new
// connections and let in-flight requests finish within the configured
// grace period (Shutdown), force-close whatever is still active if that
// period expires (Close — Shutdown alone never touches active
// connections, it only waits for them), then close pool once Shutdown/
// Close have both had their chance — never before, never concurrently.
// pool may be nil: a shutdown signal arriving before FR-1 step 6 has
// constructed one has nothing to close yet.
func gracefulShutdown(cfg *config.Config, deps runDeps, srv shutdownableServer, pool pgPool, jobSystem jobRunner, logger *slog.Logger) int {
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithDeadline(context.Background(), deps.clock.Now().Add(cfg.ShutdownGracePeriod))
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// The grace period expired with requests still in flight
			// (Failure modes table) — expected under load, not an error
			// to report, but Shutdown alone leaves those connections
			// open; force-close what's left so they're cleanly cancelled
			// rather than left for the process exit to reap out from
			// under them (FR-4).
			if closeErr := srv.Close(); closeErr != nil {
				logger.Error("force-close after shutdown timeout failed", "error", closeErr.Error())
			}
		} else {
			logger.Error("shutdown did not complete cleanly", "error", err.Error())
		}
	}

	// backend-service-lifecycle.md FR-6, amended for phase 09: the job
	// worker pool stops after the HTTP server has stopped accepting work
	// and before the shared pgxpool a running job's heartbeat/completion
	// write depends on is closed. A job still running when this grace
	// period expires is abandoned to the reaper (backend-job-queue.md
	// FR-10), not force-killed.
	if jobSystem != nil {
		jobCtx, jobCancel := context.WithDeadline(context.Background(), deps.clock.Now().Add(cfg.ShutdownGracePeriod))
		if err := jobSystem.Shutdown(jobCtx); err != nil {
			logger.Warn("job worker pool did not stop within the grace period", "error", err.Error())
		}
		jobCancel()
	}

	if pool != nil {
		pool.Close()
	}
	logger.Info("shutdown complete")
	return 0
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

	if cfg.DesktopParentPID > 0 && deps.watchParent != nil {
		if err := deps.watchParent(cfg.DesktopParentPID); err != nil {
			logger.Error("startup failed", "step", "parentwatch", "error", err.Error())
			return 1
		}
		logger.Info("startup step completed", "step", "parentwatch", "parentPID", cfg.DesktopParentPID)
	}

	poolRef := &transporthttp.PoolRef{}

	router := deps.newRouter(cfg, logger, poolRef)
	logger.Info("startup step completed", "step", "router")

	listener, err := deps.listen("tcp", cfg.BindAddress)
	if err != nil {
		logger.Error("startup failed", "step", "listen", "error", err.Error())
		return 1
	}
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		// desktop-host-process-model.md FR-2 / architecture-desktop-host.md:
		// Announces bound ephemeral port to Electron host process.
		fmt.Printf("PORT=%d\n", tcpAddr.Port)
	}
	logger.Info("startup step completed", "step", "listen", "address", listener.Addr().String())

	srv := deps.newServer(cfg, router)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()

	if err := waitForPostgres(ctx, cfg, deps.obtainPostgres, deps.postgresMaxAttempts, deps.postgresBackoff, deps.sleep, logger); err != nil {
		if ctx.Err() != nil {
			// The signal that ended this loop was a shutdown, not a
			// database failure — FR-4 requires attempting Shutdown, not
			// exiting through the ordinary FR-3 failure path below.
			return gracefulShutdown(cfg, deps, srv, nil, nil, logger)
		}
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
		if ctx.Err() != nil {
			return gracefulShutdown(cfg, deps, srv, nil, nil, logger)
		}
		msg := err.Error()
		if cfg.DatabaseURL != "" {
			// Defense in depth, matching the postgres/pool steps: today
			// internal/persistence/postgres.RunMigrations already
			// returns a fixed generic message for every connection-class
			// failure, so this guard is currently redundant — but it's
			// the one place a future change to that package's error
			// wrapping could silently reintroduce a DSN leak without
			// this step noticing.
			msg = "could not run migrations against the configured database"
		}
		logger.Error("startup failed", "step", "migrate", "error", msg)
		return 1
	}
	logger.Info("startup step completed", "step", "migrate")

	pool, repos, err := deps.newPool(ctx, cfg)
	if err != nil {
		if ctx.Err() != nil {
			return gracefulShutdown(cfg, deps, srv, nil, nil, logger)
		}
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
	if repos != nil {
		if repos.works != nil {
			poolRef.SetWorkRepository(repos.works)
		}
		if repos.collections != nil {
			poolRef.SetCollectionRepository(repos.collections)
		}
		if repos.metadataCache != nil {
			poolRef.SetMetadataCacheRepository(repos.metadataCache)
		}
		if repos.coverCache != nil {
			poolRef.SetCoverCacheRepository(repos.coverCache)
		}
		if repos.sourceRecords != nil {
			poolRef.SetSourceRecordRepository(repos.sourceRecords)
		}
		if repos.sourceRemoval != nil {
			poolRef.SetSourceRemovalService(repos.sourceRemoval)
		}
	}

	appDataDir := ""
	userConfigDirFn := deps.userConfigDir
	if userConfigDirFn == nil {
		userConfigDirFn = os.UserConfigDir
	}
	if userConfig, err := userConfigDirFn(); err == nil && userConfig != "" {
		appDataDir = filepath.Join(userConfig, "alexandryn")
	} else {
		appDataDir = filepath.Join(os.TempDir(), "alexandryn")
	}

	key, err := crypto.LoadOrCreateKey(appDataDir, func() (int, error) {
		if repos != nil && repos.sourceRecords != nil {
			return repos.sourceRecords.CountWithCredential(ctx)
		}
		return 0, nil
	}, logger)
	if err != nil {
		logger.Error("failed to load or create source credential key", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}
	cryptoSvc, err := crypto.NewService(key)
	if err != nil {
		logger.Error("failed to initialize source crypto service", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}
	cursorSubkey, err := cryptoSvc.DeriveSubkey("source-cursor-hmac-v1")
	if err != nil {
		logger.Error("failed to derive cursor subkey", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}
	poolRef.SetSourceCrypto(transporthttp.SourceCrypto{
		Encryptor: cryptoSvc,
		Codec:     sources.NewCursorCodec(cursorSubkey),
	})

	logger.Info("startup step completed", "step", "pool")

	var jobSystem jobRunner
	if deps.newJobSystem != nil {
		js, err := deps.newJobSystem(cfg, logger, pool)
		if err != nil {
			logger.Error("startup failed", "step", "jobs", "error", err.Error())
			pool.Close()
			return 1
		}
		if js != nil {
			js.Start(ctx)
			jobSystem = js
			logger.Info("startup step completed", "step", "jobs")
		}
	}

	logger.Info("ready")

	select {
	case <-ctx.Done():
		return gracefulShutdown(cfg, deps, srv, pool, jobSystem, logger)
	case err := <-serveErr:
		// The server stopped on its own, not via a shutdown signal — no
		// Shutdown was called, but FR-6's "close the pool before the
		// process exits" applies regardless of why the process is
		// exiting. The job worker pool still stops first, so a running
		// job's final write lands before the pool goes away.
		if jobSystem != nil {
			jobCtx, jobCancel := context.WithDeadline(context.Background(), deps.clock.Now().Add(cfg.ShutdownGracePeriod))
			if sErr := jobSystem.Shutdown(jobCtx); sErr != nil {
				logger.Warn("job worker pool did not stop within the grace period", "error", sErr.Error())
			}
			jobCancel()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err.Error())
			pool.Close()
			return 1
		}
		pool.Close()
		return 0
	}
}
