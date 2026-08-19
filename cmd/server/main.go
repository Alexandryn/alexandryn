// Command server is Alexandryn's backend entry point.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/logging"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// postgresReadyMaxAttempts and postgresReadyBackoff bound FR-1 step 5's
// "wait for Postgres to become reachable" retry (FR-3's sole retry
// exception). The spec's own Open questions leave the exact number to
// backend-persistence.md, which is better positioned to measure real
// spawn/start timing than this spec; ~30 seconds is a provisional guess
// pending that number, not a considered decision — revisit here once
// backend-persistence.md fixes one.
//
// postgresConnectAttemptTimeout bounds each individual connect attempt:
// without it, a host that accepts the TCP handshake but never completes
// Postgres's own startup message exchange could block a single attempt
// far longer than postgresReadyBackoff implies, turning the "~30 second"
// budget above into an unbounded one (found in Checkpoint F's security
// review — DATABASE_URL is operator-supplied, constitution §4).
const (
	postgresReadyMaxAttempts      = 30
	postgresReadyBackoff          = time.Second
	postgresConnectAttemptTimeout = 5 * time.Second
)

func main() {
	configPath := flag.String("config", "", "path to config.toml (default: the OS's per-user config directory)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	os.Exit(run(ctx, runDeps{
		loadConfig: func() (*config.Config, error) {
			return config.Load(*configPath, os.ReadFile, os.UserConfigDir)
		},
		newLogger: func(cfg *config.Config) *slog.Logger {
			return logging.New(cfg.LogLevel, os.Stdout)
		},
		newRouter: newProductionRouter,
		listen:    net.Listen,
		newServer: func(cfg *config.Config, handler http.Handler) shutdownableServer {
			return transporthttp.NewServer(cfg, handler)
		},
		clock:               realClock{},
		obtainPostgres:      obtainPostgres,
		postgresMaxAttempts: postgresReadyMaxAttempts,
		postgresBackoff:     postgresReadyBackoff,
		sleep:               sleepOrDone,
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			return postgres.Migrate(ctx, cfg.DatabaseURL.Reveal())
		},
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, error) {
			return postgres.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBPoolMaxConns)
		},
		stderr: os.Stderr,
	}))
}

// obtainPostgres makes one attempt at FR-1 step 5: spawn a bundled
// instance when no DATABASE_URL is configured (the Electron-hosted
// target's production path), or connect directly to the one configured
// (the Electron target's dev/CI/test override, and the container-hosted
// target's normal production path, ADR 0015) — exactly one of the two,
// chosen by postgres.SelectStartupPath.
func obtainPostgres(ctx context.Context, cfg *config.Config) error {
	return postgres.SelectStartupPath(ctx, cfg.DatabaseURL.Reveal(), spawnPostgres, connectPostgres(cfg))
}

// spawnPostgres would initialize the data directory and spawn a bundled,
// platform-appropriate PostgreSQL instance (backend-persistence.md FR-8)
// for the Electron-hosted production target. That mechanics isn't built
// yet — internal/persistence/postgres currently only has the portable
// argument-list builder (PostgresArgs) and branch-selection logic
// (SelectStartupPath), not a real process spawn — tracked as T25 in
// tasks/plan.md (production spawn failure, macOS/Linux/Windows E2E).
// Failing loudly here, rather than silently no-op'ing, is FR-3's own
// requirement: a startup step that can't do its job fails, it doesn't
// pretend to succeed.
func spawnPostgres(ctx context.Context) error {
	return errors.New("spawning a managed PostgreSQL instance is not implemented yet (backend-persistence.md FR-8, tracked as T25)")
}

// connectPostgres returns a connect function for postgres.SelectStartupPath:
// one attempt at establishing (and immediately closing) a real connection
// to cfg.DatabaseURL — proof of reachability, per FR-1 step 5, without
// building the long-lived pool step 6 owns. Bounded by
// postgresConnectAttemptTimeout so a host that accepts the TCP connection
// but never completes Postgres's own handshake can't block a single
// attempt indefinitely.
func connectPostgres(cfg *config.Config) func(ctx context.Context) error {
	return connectPostgresWithTimeout(cfg, postgresConnectAttemptTimeout)
}

// connectPostgresWithTimeout is connectPostgres with an injectable
// timeout, so a test can prove the bound is actually applied without
// waiting out the real production duration.
func connectPostgresWithTimeout(cfg *config.Config, timeout time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		conn, err := pgx.Connect(attemptCtx, cfg.DatabaseURL.Reveal())
		if err != nil {
			return err
		}
		return conn.Close(attemptCtx)
	}
}

// sleepOrDone waits for d or ctx's cancellation, whichever comes first —
// production's real implementation of runDeps.sleep, so a shutdown signal
// arriving during the Postgres-reachability retry loop's backoff wait
// interrupts it immediately instead of being absorbed by a real
// time.Sleep that ignores ctx entirely.
func sleepOrDone(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// newProductionRouter assembles the real router: /healthz and /readyz
// wired to poolRef, no /api/v1 routes yet (phase 03 registers none), and
// the embedded web/dist build as the SPA-fallback catch-all — wrapped by
// the middleware chain in architecture-backend.md FR-6's fixed order
// (recovery, limits, logging, routing).
func newProductionRouter(cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", transporthttp.Healthz(poolRef))
	mux.Handle("/readyz", transporthttp.Readyz(poolRef))
	mux.Handle("/api/v1/", transporthttp.NotFoundHandler())
	mux.Handle("/", transporthttp.DefaultStaticHandler())

	return transporthttp.Chain(mux,
		transporthttp.Recovery(logger, newCorrelationID),
		transporthttp.Limits(cfg.HTTPMaxBodyBytes),
		transporthttp.Logging(logger, newCorrelationID),
	)
}

// newCorrelationID generates a random per-request correlation ID
// (backend-errors-and-logging.md FR-7) — 16 bytes of crypto/rand, hex
// encoded, no new dependency for something the standard library already
// does.
func newCorrelationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read failing means the OS entropy source itself is
		// broken — not a condition this process can meaningfully recover
		// from or fall back on.
		panic("cmd/server: crypto/rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
