package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/idgen"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/pairing"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/time/rate"
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
	newRouter  func(ctx context.Context, cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef, publicLimiter *auth.IPRateLimiter) http.Handler
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
func gracefulShutdown(cfg *config.Config, deps runDeps, srv shutdownableServer, redirectSrv *http.Server, pool pgPool, jobSystem jobRunner, logger *slog.Logger) int {
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithDeadline(context.Background(), deps.clock.Now().Add(cfg.ShutdownGracePeriod))
	defer cancel()

	// The :80 redirect/ACME listener (if any) drains alongside the main
	// server, bounded by the same grace period (FR-11), concurrently.
	var shutdownWG sync.WaitGroup

	if redirectSrv != nil {
		shutdownWG.Add(1)
		go func() {
			defer shutdownWG.Done()
			if err := redirectSrv.Shutdown(shutdownCtx); err != nil {
				logger.Warn("the :80 redirect listener did not drain within the grace period", "error", err.Error())
				_ = redirectSrv.Close()
			}
		}()
	}

	shutdownWG.Add(1)
	go func() {
		defer shutdownWG.Done()
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
	}()

	shutdownWG.Wait()

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

	// The per-user data directory (architecture-persistence.md FR-1) —
	// resolved once, used for the ACME certificate cache and the source
	// credential key.
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

	if cfg.DesktopParentPID > 0 && deps.watchParent != nil {
		if err := deps.watchParent(cfg.DesktopParentPID); err != nil {
			logger.Error("startup failed", "step", "parentwatch", "error", err.Error())
			return 1
		}
		logger.Info("startup step completed", "step", "parentwatch", "parentPID", cfg.DesktopParentPID)
	}

	poolRef := &transporthttp.PoolRef{}

	// The unauthenticated health-probe rate limiter (backend-network-transport.md
	// FR-8). Its per-IP map is evicted on a ticker bound to ctx — without
	// that the map only grows.
	publicLimiter := auth.NewIPRateLimiter(rate.Every(time.Second/2), 60, 10*time.Minute)
	publicLimiter.StartEviction(ctx)

	router := deps.newRouter(ctx, cfg, logger, poolRef, publicLimiter)
	logger.Info("startup step completed", "step", "router")

	listener, err := deps.listen("tcp", cfg.BindAddress)
	if err != nil {
		logger.Error("startup failed", "step", "listen", "error", err.Error())
		return 1
	}
	boundPort := ""
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		// desktop-host-process-model.md FR-2 / architecture-desktop-host.md:
		// Announces bound ephemeral port to Electron host process.
		fmt.Printf("PORT=%d\n", tcpAddr.Port)
		boundPort = strconv.Itoa(tcpAddr.Port)
	}
	logger.Info("startup step completed", "step", "listen", "address", listener.Addr().String())

	// TLS mode + the :80 redirect listener (ADR 0028 §1/§2/§3,
	// backend-network-transport.md FR-2/FR-3). config.TLSMode()/Reachability()
	// were fixed by validateBindAddress from the address class plus the
	// certificate/ACME state — never a flag.
	var acmeManager *autocert.Manager
	switch cfg.TLSMode() {
	case "static":
		tlsCfg := transporthttp.NewTLSConfig()
		tlsCfg.Certificates = []tls.Certificate{*cfg.TLSCertificate()}
		listener = tls.NewListener(listener, tlsCfg)
		logger.Info("startup step completed", "step", "tls", "mode", "static")
	case "acme":
		cacheDir := cfg.ACMECacheDir
		if cacheDir == "" {
			cacheDir = filepath.Join(appDataDir, "acme")
		}
		acmeManager = transporthttp.NewACMEManager(cfg, cacheDir)
		tlsCfg := transporthttp.NewTLSConfig()
		tlsCfg.GetCertificate = acmeManager.GetCertificate
		listener = tls.NewListener(listener, tlsCfg)
		caURL := "https://acme-v02.api.letsencrypt.org/directory (Let's Encrypt, autocert default)"
		if acmeManager.Client != nil && acmeManager.Client.DirectoryURL != "" {
			caURL = acmeManager.Client.DirectoryURL
		}
		// The cache directory path is NOT logged — it is home-relative
		// (constitution §8, CLAUDE.md reflex). The operator sets or knows
		// ACME_CACHE_DIR; whether it is the default or explicit is all the
		// log needs to say.
		cacheDirKind := "default (acme/ under the data directory)"
		if cfg.ACMECacheDir != "" {
			cacheDirKind = "ACME_CACHE_DIR"
		}
		logger.Info("startup step completed", "step", "tls", "mode", "acme",
			"domain", cfg.ACMEDomain, "cacheDir", cacheDirKind, "caDirectoryURL", caURL)
	default:
		logger.Info("startup step completed", "step", "tls", "mode", "none")
	}

	// Every public bind (static or ACME) gets a :80 HTTP->HTTPS redirect
	// listener so http:// is not connection-refused; in ACME mode it also
	// serves the HTTP-01 challenge. It never serves application content.
	var redirectSrv *http.Server
	if cfg.Reachability() == "public" {
		tlsPort := boundPort
		if tlsPort == "" {
			if _, p, err := net.SplitHostPort(cfg.BindAddress); err == nil {
				tlsPort = p
			}
		}
		// cfg.NamedBindHost() reuses validateBindAddress's own
		// DNS-name-vs-IP-literal classification rather than re-deriving
		// it here (a second copy of that check is a drift risk on a
		// redirect-target, Host-header-adjacent surface).
		canonicalHost := cfg.ACMEDomain
		if canonicalHost == "" {
			canonicalHost = cfg.NamedBindHost()
		}
		h := transporthttp.HTTPSRedirect(canonicalHost, tlsPort)
		if acmeManager != nil {
			h = acmeManager.HTTPHandler(h)
		}
		// The same health-probe limiter and correlation/recovery/security
		// layers the main router uses — one rate-limit policy, one map,
		// not a second un-synchronized copy for this listener alone.
		// SecurityHeaders applies here too (its own doc: "every response
		// on every bind"); Logging is skipped — this listener never
		// serves anything but a redirect or an ACME challenge, neither
		// carrying a correlation ID a client would ever see.
		h = transporthttp.PublicRateLimit(publicLimiter, func(string) bool { return true })(h)
		h = transporthttp.SecurityHeaders()(h)
		h = transporthttp.Recovery(logger, newCorrelationID)(h)

		rl, lerr := deps.listen("tcp", ":80")
		if lerr != nil {
			if cfg.TLSMode() == "acme" {
				logger.Error("startup failed", "step", "listen-80", "error", lerr.Error())
				return 1
			}
			logger.Warn("could not bind :80 for the HTTP->HTTPS redirect; http:// will be connection-refused", "error", lerr.Error())
		} else {
			redirectSrv = &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
			go func() { _ = redirectSrv.Serve(rl) }()
			logger.Info("startup step completed", "step", "listen-80")
		}
	}

	srv := deps.newServer(cfg, router)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()

	if err := waitForPostgres(ctx, cfg, deps.obtainPostgres, deps.postgresMaxAttempts, deps.postgresBackoff, deps.sleep, logger); err != nil {
		if ctx.Err() != nil {
			// The signal that ended this loop was a shutdown, not a
			// database failure — FR-4 requires attempting Shutdown, not
			// exiting through the ordinary FR-3 failure path below.
			return gracefulShutdown(cfg, deps, srv, redirectSrv, nil, nil, logger)
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
			return gracefulShutdown(cfg, deps, srv, redirectSrv, nil, nil, logger)
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
			return gracefulShutdown(cfg, deps, srv, redirectSrv, nil, nil, logger)
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
		if repos.libraryEntries != nil && repos.sourceOfferings != nil && repos.sourceRecords != nil {
			contentSourceResolver := transporthttp.NewSourceProviderResolver(repos.sourceRecords, poolRef, logger)
			contentResolver := content.NewResolver(repos.libraryEntries, repos.sourceOfferings, contentSourceResolver)
			poolRef.SetReaderContentCache(content.NewCache(contentResolver.Load))
		}
		if repos.readingProgress != nil && repos.transactor != nil {
			poolRef.SetReadingAPI(transporthttp.ReadingAPI{
				Progress:       repos.readingProgress,
				Bookmarks:      repos.bookmarks,
				Highlights:     repos.highlights,
				Preferences:    repos.readingPreferences,
				Editions:       repos.editions,
				LibraryEntries: repos.libraryEntries,
				Transactor:     repos.transactor,
				IDs:            idgen.New(),
				Export:         repos.readingExport,
			})
		}
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
	jwtSubkey, err := cryptoSvc.DeriveSubkey("jwt-signing-secret-v1")
	if err != nil {
		logger.Error("failed to derive jwt subkey", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}
	mfaSubkey, err := cryptoSvc.DeriveSubkey("mfa-totp-master-v1")
	if err != nil {
		logger.Error("failed to derive mfa subkey", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}
	pairingEncSubkey, err := cryptoSvc.DeriveSubkey("pairing-code-enc-v1")
	if err != nil {
		logger.Error("failed to derive pairing code enc subkey", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}
	pairingIndexSubkey, err := cryptoSvc.DeriveSubkey("pairing-code-index-v1")
	if err != nil {
		logger.Error("failed to derive pairing code index subkey", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}
	enrolmentGrantSubkey, err := cryptoSvc.DeriveSubkey("enrolment-grant-v1")
	if err != nil {
		logger.Error("failed to derive enrolment grant subkey", "error", err.Error())
		if pool != nil {
			pool.Close()
		}
		return 1
	}

	poolRef.SetSourceCrypto(transporthttp.SourceCrypto{
		Encryptor: cryptoSvc,
		Codec:     sources.NewCursorCodec(cursorSubkey),
	})

	if repos != nil {
		var enrolmentSigner *auth.EnrolmentGrantSigner
		if repos.pool != nil && repos.pairedDevices != nil && repos.networkSettings != nil {
			pairingSessionRepo, err := postgres.NewPairingSessionRepository(repos.pool, pairingEncSubkey, pairingIndexSubkey)
			if err != nil {
				logger.Error("failed to create pairing session repository", "error", err.Error())
				if pool != nil {
					pool.Close()
				}
				return 1
			}
			verifier := postgres.NewPairingVerifier(repos.transactor, pairingSessionRepo, repos.pairedDevices)
			enrolmentSigner = auth.NewEnrolmentGrantSigner(enrolmentGrantSubkey, "alexandryn", idgen.New())

			scheme := "http"
			if cfg.TLSMode() == "static" || cfg.TLSMode() == "acme" {
				scheme = "https"
			}

			poolRef.SetNetworkAPI(transporthttp.NetworkAPI{
				PairingSessions:    pairingSessionRepo,
				PairedDevices:      repos.pairedDevices,
				NetworkSettings:    repos.networkSettings,
				EnrolmentGrantJTIs: repos.enrolmentGrantJTIs,
				Verifier:           verifier,
				GrantSigner:        enrolmentSigner,
				CodeGen:            pairing.GeneratePairingCode,
				PairingSecret:      cfg.DevicePairingSecret.Reveal(),
				ServerAddress:      resolveServerAddress(cfg),
				HostName:           "alexandryn.local",
				Scheme:             scheme,
				IDs:                idgen.New(),
				Now:                time.Now,
				Logger:             logger,
				InfoProvider:       fallbackNetworkInfo(cfg),
			})
		}

		if repos.users != nil {
			// The auth-endpoint brute-force limiter — evicted on a ctx-bound
			// ticker, same as publicLimiter (previously this map only grew).
			authLimiter := auth.NewIPRateLimiter(rate.Every(time.Second/5), 10, 15*time.Minute)
			authLimiter.StartEviction(ctx)
			poolRef.SetAuthAPI(transporthttp.AuthAPI{
				Users:              repos.users,
				Credentials:        repos.credentials,
				RefreshTokens:      repos.refreshTokens,
				MFA:                repos.mfa,
				PasswordResets:     repos.passwordResets,
				Libraries:          repos.libraries,
				LibraryMemberships: repos.libraryMemberships,
				LibraryInvitations: repos.libraryInvitations,
				Hasher:             auth.NewArgon2idPasswordHasher(auth.DefaultArgon2idParams()),
				Signer:             auth.NewJWTSigner(jwtSubkey, "alexandryn"),
				TOTPEngine:         auth.NewTOTPEngine("Alexandryn"),
				Limiter:            authLimiter,
				MasterKey:          mfaSubkey,
				IDs:                idgen.New(),
				PairedDevices:      repos.pairedDevices,
				NetworkSettings:    repos.networkSettings,
				EnrolmentGrantJTIs: repos.enrolmentGrantJTIs,
				EnrolmentSigner:    enrolmentSigner,
				Logger:             logger,
			})
		}
		if repos.importCandidates != nil {
			poolRef.SetImportCandidateRepository(repos.importCandidates)
		}
		if repos.importerService != nil {
			poolRef.SetImporterService(repos.importerService)
		}
		if repos.pairedDevices != nil && repos.readingSync != nil && repos.readingProgress != nil {
			poolRef.SetSyncAPI(transporthttp.SyncAPI{
				Devices:        repos.pairedDevices,
				Progress:       repos.readingProgress,
				LibraryEntries: repos.libraryEntries,
				SyncStore:      repos.readingSync,
				Transactor:     repos.transactor,
				IDs:            idgen.New(),
				Now:            time.Now,
			})
		}
	}

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
			if jsConcrete, ok := js.(*jobs.System); ok && repos != nil && repos.importCandidates != nil {
				sourceResolver := transporthttp.NewSourceProviderResolver(repos.sourceRecords, poolRef, logger)
				olClient := openlibrary.NewClient("", cfg.OpenLibraryUserAgent, logger, nil, nil)
				matcher := importer.NewMatcher(repos.importCandidates, olClient)
				importHandler := importer.NewJobHandler(sourceResolver, repos.importCandidates, matcher, repos.importerService)
				jsConcrete.Queue().Register("import", 3, importHandler)

				sourceChecker := transporthttp.NewSourceCheckerAdapter(repos.sources)
				jobEnqueuer := transporthttp.NewJobQueueEnqueuer(jsConcrete.Queue())
				discoveryCoord := importer.NewDiscoveryCoordinator(sourceChecker, sourceResolver, repos.importCandidates, jobEnqueuer, idgen.New())
				poolRef.SetDiscoveryCoordinator(discoveryCoord)
			}
			js.Start(ctx)
			jobSystem = js
			logger.Info("startup step completed", "step", "jobs")
		}
	}

	if repos != nil && repos.networkSweep != nil {
		repos.networkSweep.Start(ctx, 5*time.Minute, logger)
	}

	logger.Info("ready")

	select {
	case <-ctx.Done():
		return gracefulShutdown(cfg, deps, srv, redirectSrv, pool, jobSystem, logger)
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
