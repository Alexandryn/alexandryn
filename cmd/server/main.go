// Command server is Alexandryn's backend entry point.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/deskhost/parentwatch"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/idgen"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/logging"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/observability"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"golang.org/x/time/rate"
)

var (
	gitCommit       = "dev"
	buildTime       = ""
	serverStartTime = time.Now()
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
		obtainPostgres:      newObtainPostgres(),
		postgresMaxAttempts: postgresReadyMaxAttempts,
		postgresBackoff:     postgresReadyBackoff,
		sleep:               sleepOrDone,
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			return postgres.Migrate(ctx, cfg.DatabaseURL.Reveal())
		},
		userConfigDir: os.UserConfigDir,
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
			pool, err := postgres.NewPool(ctx, cfg.DatabaseURL.Reveal(), cfg.DBPoolMaxConns)
			if err != nil {
				return nil, nil, err
			}
			return pool, newRepositories(pool), nil
		},
		newJobSystem: func(cfg *config.Config, logger *slog.Logger, pool pgPool) (jobRunner, error) {
			// The pool run holds is the pgPool interface; the job
			// subsystem needs the concrete *pgxpool.Pool it shares with
			// every repository (backend-job-queue.md FR-1 — one pool, not
			// a second). In production newPool always returns exactly
			// that; a mismatch is a wiring bug worth failing loudly on.
			pgxPool, ok := pool.(*pgxpool.Pool)
			if !ok {
				return nil, fmt.Errorf("job worker pool needs a *pgxpool.Pool, got %T", pool)
			}
			return jobs.NewSystem(pgxPool, idgen.New(), jobs.SystemClock{}, logger, jobs.Config{
				ShutdownGracePeriod: cfg.ShutdownGracePeriod,
			}), nil
		},
		watchParent: parentwatch.Watch,
		stderr:      os.Stderr,
	}))
}

// newObtainPostgres (spawn.go, spawn_darwin.go) is FR-1 step 5's real,
// per-platform implementation: spawn a bundled instance when no
// DATABASE_URL is configured (the Electron-hosted target's production
// path), or connect directly to the one configured (the Electron
// target's dev/CI/test override, and the container-hosted target's
// normal production path, ADR 0015) — exactly one of the two, chosen by
// postgres.SelectStartupPath.

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
func newProductionRouter(ctx context.Context, cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef, publicLimiter *auth.IPRateLimiter) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", transporthttp.Healthz(poolRef))
	mux.Handle("/readyz", transporthttp.Readyz(poolRef))

	workRepo := transporthttp.NewLazyWorkRepository(poolRef)
	mux.Handle("GET /api/v1/library", transporthttp.LibraryHandler(workRepo))
	mux.Handle("GET /api/v1/works/{id}", transporthttp.WorkDetailHandler(workRepo))

	collRepo := transporthttp.NewLazyCollectionRepository(poolRef)
	idGen := idgen.New()
	mux.Handle("GET /api/v1/collections", transporthttp.ListCollectionsHandler(collRepo))
	mux.Handle("POST /api/v1/collections", transporthttp.CreateCollectionHandler(collRepo, idGen))
	mux.Handle("GET /api/v1/collections/{id}", transporthttp.GetCollectionHandler(collRepo))
	mux.Handle("PATCH /api/v1/collections/{id}", transporthttp.RenameCollectionHandler(collRepo))
	mux.Handle("DELETE /api/v1/collections/{id}", transporthttp.DeleteCollectionHandler(collRepo))
	mux.Handle("POST /api/v1/collections/{id}/works", transporthttp.AddWorkToCollectionHandler(collRepo, time.Now))
	mux.Handle("DELETE /api/v1/collections/{id}/works/{workId}", transporthttp.RemoveWorkFromCollectionHandler(collRepo))

	openLibraryClient := openlibrary.NewClient("", cfg.OpenLibraryUserAgent, logger, nil, nil)
	metadataCache := transporthttp.NewLazyMetadataCacheRepository(poolRef)
	coverCache := transporthttp.NewLazyCoverCacheRepository(poolRef)

	mux.Handle("GET /api/v1/discover", transporthttp.DiscoverSearchHandler(openLibraryClient))
	mux.Handle("GET /api/v1/discover/works/{openLibraryId}", transporthttp.DiscoverWorkDetailHandler(openLibraryClient, metadataCache))
	mux.Handle("GET /api/v1/discover/covers/{coverId}", transporthttp.DiscoverCoverHandler(openLibraryClient, coverCache))

	sourceRepo := transporthttp.NewLazySourceRecordRepository(poolRef)
	sourceSem := sources.NewSemaphore(sources.DefaultOutboundLimit)

	adminOnly := transporthttp.RequireRole(domain.RoleAdmin)

	mux.Handle("POST /api/v1/sources", adminOnly(transporthttp.CreateSourceHandler(sourceRepo, poolRef, sourceSem, idGen, logger)))
	mux.Handle("GET /api/v1/sources", adminOnly(transporthttp.ListSourcesHandler(sourceRepo)))
	mux.Handle("GET /api/v1/sources/{id}", adminOnly(transporthttp.GetSourceHandler(sourceRepo)))
	mux.Handle("PATCH /api/v1/sources/{id}", adminOnly(transporthttp.UpdateSourceHandler(sourceRepo, poolRef, sourceSem, logger)))
	mux.Handle("DELETE /api/v1/sources/{id}", adminOnly(transporthttp.DeleteSourceHandler(sourceRepo, poolRef)))
	mux.Handle("POST /api/v1/sources/{id}/health-check", adminOnly(transporthttp.HealthCheckSourceHandler(sourceRepo, poolRef, sourceSem, logger)))
	mux.Handle("GET /api/v1/sources/{id}/browse", adminOnly(transporthttp.BrowseSourceHandler(sourceRepo, poolRef, sourceSem, logger)))
	mux.Handle("GET /api/v1/sources/{id}/search", adminOnly(transporthttp.SearchSourceHandler(sourceRepo, poolRef, sourceSem, logger)))

	candRepo := transporthttp.NewLazyImportCandidateRepository(poolRef)
	discoveryRunner := transporthttp.NewLazyDiscoveryRunner(poolRef)
	importerSvc := transporthttp.NewLazyImporterService(poolRef)
	ingestAuth := transporthttp.LazyRequireIngestPermission(poolRef)

	mux.Handle("POST /api/v1/import/discover", ingestAuth(transporthttp.ImportDiscoverHandler(discoveryRunner)))
	mux.Handle("GET /api/v1/import/candidates", ingestAuth(transporthttp.ImportCandidatesListHandler(candRepo)))
	mux.Handle("POST /api/v1/import/candidates/{id}/confirm", ingestAuth(transporthttp.ImportCandidateConfirmHandler(importerSvc, candRepo, openLibraryClient)))
	mux.Handle("POST /api/v1/import/candidates/{id}/reject", ingestAuth(transporthttp.ImportCandidateRejectHandler(importerSvc, candRepo)))

	mux.Handle("GET /api/v1/library/editions/{editionId}/reader/content/{path...}", transporthttp.ReaderContentHandler(poolRef, logger))

	syncMW := transporthttp.LazySyncMiddleware(poolRef)

	mux.Handle("GET /api/v1/reading/works/{workId}/progress", transporthttp.ReadingProgressGetHandler(poolRef))
	mux.Handle("POST /api/v1/reading/works/{workId}/progress", transporthttp.Chain(transporthttp.ReadingProgressReportHandler(poolRef, time.Now), syncMW))
	mux.Handle("GET /api/v1/reading/editions/{editionId}/bookmarks", transporthttp.ReadingBookmarksListHandler(poolRef))
	mux.Handle("POST /api/v1/reading/editions/{editionId}/bookmarks", transporthttp.Chain(transporthttp.ReadingBookmarkCreateHandler(poolRef, time.Now), syncMW))
	mux.Handle("DELETE /api/v1/reading/bookmarks/{bookmarkId}", transporthttp.Chain(transporthttp.ReadingBookmarkDeleteHandler(poolRef), syncMW))
	mux.Handle("GET /api/v1/reading/editions/{editionId}/highlights", transporthttp.ReadingHighlightsListHandler(poolRef))
	mux.Handle("POST /api/v1/reading/editions/{editionId}/highlights", transporthttp.Chain(transporthttp.ReadingHighlightCreateHandler(poolRef, time.Now), syncMW))
	mux.Handle("PATCH /api/v1/reading/highlights/{highlightId}", transporthttp.Chain(transporthttp.ReadingHighlightPatchHandler(poolRef), syncMW))
	mux.Handle("DELETE /api/v1/reading/highlights/{highlightId}", transporthttp.Chain(transporthttp.ReadingHighlightDeleteHandler(poolRef), syncMW))
	mux.Handle("GET /api/v1/reading/preferences", transporthttp.ReadingPreferencesGetHandler(poolRef))
	mux.Handle("PUT /api/v1/reading/preferences", transporthttp.Chain(transporthttp.ReadingPreferencesPutHandler(poolRef), syncMW))
	mux.Handle("GET /api/v1/reading/export", transporthttp.ReadingExportHandler(poolRef, logger, time.Now))

	// Auth routes (Phase 12). /auth/setup and /auth/login are, like
	// /network/pair/verify, unauthenticated state-changing POST routes
	// (IsPublicPath) — OriginValidation applies to all three for the same
	// reason (backend-network-transport.md FR-7, ADR 0028 §5).
	allowedOrigins := computeAllowedOrigins(cfg)
	originValidated := func(h http.Handler) http.Handler {
		return transporthttp.Chain(h, transporthttp.OriginValidation(allowedOrigins))
	}

	mux.Handle("GET /api/v1/auth/setup/status", transporthttp.LazySetupStatusHandler(poolRef))
	mux.Handle("POST /api/v1/auth/setup", originValidated(transporthttp.LazySetupHandler(poolRef)))
	mux.Handle("POST /api/v1/auth/login", originValidated(transporthttp.LazyLoginHandler(poolRef)))
	mux.Handle("POST /api/v1/auth/refresh", transporthttp.LazyRefreshHandler(poolRef))
	mux.Handle("POST /api/v1/auth/logout", transporthttp.LazyLogoutHandler(poolRef))
	mux.Handle("POST /api/v1/auth/password-reset/request", transporthttp.LazyPasswordResetRequestHandler(poolRef))
	mux.Handle("POST /api/v1/auth/password-reset/confirm", transporthttp.LazyPasswordResetConfirmHandler(poolRef))
	mux.Handle("POST /api/v1/auth/mfa/totp/setup", transporthttp.LazyTOTPSetupHandler(poolRef))
	mux.Handle("POST /api/v1/auth/mfa/totp/confirm", transporthttp.LazyTOTPConfirmHandler(poolRef))
	mux.Handle("POST /api/v1/auth/mfa/totp/verify", transporthttp.LazyTOTPVerifyHandler(poolRef))
	mux.Handle("POST /api/v1/auth/mfa/totp/disable", transporthttp.LazyTOTPDisableHandler(poolRef))

	// Multi-Library routes (Phase 12)
	mux.Handle("GET /api/v1/libraries", transporthttp.LazyListLibrariesHandler(poolRef))
	mux.Handle("POST /api/v1/libraries", transporthttp.LazyCreateLibraryHandler(poolRef))
	mux.Handle("GET /api/v1/libraries/{id}", transporthttp.LazyGetLibraryHandler(poolRef))
	mux.Handle("PATCH /api/v1/libraries/{id}", transporthttp.LazyUpdateLibraryHandler(poolRef))
	mux.Handle("DELETE /api/v1/libraries/{id}", transporthttp.LazyDeleteLibraryHandler(poolRef))
	mux.Handle("GET /api/v1/libraries/{id}/members", transporthttp.LazyListMembersHandler(poolRef))
	mux.Handle("POST /api/v1/libraries/{id}/invitations", transporthttp.LazyCreateInvitationHandler(poolRef))
	mux.Handle("POST /api/v1/invitations/{token}/accept", transporthttp.LazyAcceptInvitationHandler(poolRef))

	// Network & Pairing routes (Phase 13)
	pairInitiateLimiter := auth.NewIPRateLimiter(rate.Every(time.Minute/5), 3, 15*time.Minute)
	pairInitiateLimiter.StartEviction(ctx)
	pairVerifyLimiter := auth.NewIPRateLimiter(rate.Every(time.Minute/10), 5, 15*time.Minute)
	pairVerifyLimiter.StartEviction(ctx)

	initiateHandler := transporthttp.Chain(
		transporthttp.LazyInitiatePairingHandler(poolRef),
		transporthttp.PublicRateLimit(pairInitiateLimiter, func(string) bool { return true }),
	)
	mux.Handle("POST /api/v1/network/pair/initiate", initiateHandler)

	verifyHandler := transporthttp.Chain(
		transporthttp.LazyVerifyPairingHandler(poolRef),
		transporthttp.PublicRateLimit(pairVerifyLimiter, func(string) bool { return true }),
		transporthttp.OriginValidation(allowedOrigins),
	)
	mux.Handle("POST /api/v1/network/pair/verify", verifyHandler)

	mux.Handle("GET /api/v1/network/pair/{id}/qr", transporthttp.LazyPairingQRHandler(poolRef))
	mux.Handle("GET /api/v1/network/status", transporthttp.LazyNetworkStatusHandler(poolRef, fallbackNetworkInfo(cfg)))
	mux.Handle("PATCH /api/v1/network/settings", transporthttp.LazyUpdateNetworkSettingsHandler(poolRef))
	mux.Handle("DELETE /api/v1/network/pair/{id}", transporthttp.LazyDeletePairingHandler(poolRef))

	// Device Management & Sync routes (Phase 14)
	mux.Handle("GET /api/v1/devices", transporthttp.Chain(transporthttp.LazyListDevicesHandler(poolRef), syncMW))
	mux.Handle("DELETE /api/v1/devices/{id}", transporthttp.Chain(transporthttp.LazyRevokeDeviceHandler(poolRef), syncMW))
	mux.Handle("GET /api/v1/sync/reading", transporthttp.Chain(transporthttp.LazySyncReadingHandler(poolRef), syncMW))
	mux.Handle("POST /api/v1/sync/progress", transporthttp.Chain(transporthttp.LazySyncProgressHandler(poolRef), syncMW))

	mux.Handle("GET /api/bootstrap", transporthttp.LazyBootstrapHandler(poolRef))

	// Observability & Activity routes (Phase 15)
	mux.Handle("GET /api/v1/diagnostics", adminOnly(transporthttp.LazyDiagnosticsHandler(poolRef, serverStartTime, gitCommit, buildTime)))
	mux.Handle("GET /api/v1/activity/events", adminOnly(transporthttp.LazyActivityEventsHandler(poolRef)))
	mux.Handle("POST /api/v1/activity/pause-all", adminOnly(transporthttp.LazyActivityPauseAllHandler(poolRef)))
	mux.Handle("POST /api/v1/activity/jobs/{id}/cancel", adminOnly(transporthttp.LazyActivityJobCancelHandler(poolRef)))
	mux.Handle("POST /api/v1/activity/jobs/{id}/retry", adminOnly(transporthttp.LazyActivityJobRetryHandler(poolRef)))
	mux.Handle("POST /api/v1/activity/jobs/clear-completed", adminOnly(transporthttp.LazyActivityClearCompletedHandler(poolRef)))
	mux.Handle("GET /api/v1/library/finished", transporthttp.LazyFinishedWorksHandler(poolRef))
	mux.Handle("GET /api/v1/library/leaderboard", transporthttp.LazyLibraryLeaderboardHandler(poolRef))

	mux.Handle("/api/v1/", transporthttp.NotFoundHandler())

	mux.Handle("/", transporthttp.DefaultStaticHandler())

	metricsReg := observability.NewRegistry()
	poolRef.SetMetricsRegistry(metricsReg)

	// The middleware chain, outermost-in (backend-http-transport.md FR-1
	// as amended for ADR 0028, architecture-backend.md FR-6, backend-observability.md FR-4):
	//   recovery -> limits -> logging -> metrics -> security headers (all binds) ->
	//   HSTS (in-process TLS only) -> global rate limit (health probes) ->
	//   CORS -> auth -> routing.
	return transporthttp.Chain(mux,
		transporthttp.Recovery(logger, newCorrelationID),
		transporthttp.Limits(cfg.HTTPMaxBodyBytes),
		transporthttp.Logging(logger, newCorrelationID),
		transporthttp.Metrics(metricsReg),
		transporthttp.SecurityHeaders(),
		transporthttp.HSTS(cfg.TLSMode() == "static" || cfg.TLSMode() == "acme"),
		transporthttp.PublicRateLimit(publicLimiter, transporthttp.HealthProbePath),
		transporthttp.CORS(cfg.CORSAllowedOrigins),
		transporthttp.LazyAuthMiddleware(poolRef),
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
