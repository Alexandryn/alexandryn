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
	"github.com/Alexandryn/alexandryn/internal/observability"
	"github.com/Alexandryn/alexandryn/internal/pairing"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/reader/content"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/time/rate"
)

// pgPool is the minimal interface run needs from the database pool:
// transporthttp.Pinger for /readyz, plus Close for graceful shutdown.
// A real *pgxpool.Pool implements both natively; tests use a fake.
type pgPool interface {
	transporthttp.Pinger
	Close()
}

// shutdownableServer is the minimal interface run needs from the HTTP server:
// Serve to start, Shutdown to stop cleanly, and Close to force-close active
// connections if the grace period expires.
type shutdownableServer interface {
	Serve(l net.Listener) error
	Shutdown(ctx context.Context) error
	Close() error
}

// jobRunner is the minimal surface run needs from the background job
// worker pool: start it once PostgreSQL is reachable, stop it during
// graceful shutdown between the HTTP server stopping and the shared pool closing.
type jobRunner interface {
	Start(ctx context.Context)
	Shutdown(ctx context.Context) error
}

// clock provides the current time for computing shutdown grace period deadlines.
type clock interface {
	Now() time.Time
}

// realClock is production's clock: the real wall clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// runDeps carries run's dependencies as struct fields to enable test injection.
type runDeps struct {
	loadConfig func() (*config.Config, error)
	newLogger  func(cfg *config.Config) *slog.Logger
	newRouter  func(ctx context.Context, cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef, publicLimiter *auth.IPRateLimiter) http.Handler
	listen     func(network, address string) (net.Listener, error)
	newServer  func(cfg *config.Config, handler http.Handler) shutdownableServer
	clock      clock

	// obtainPostgres makes one attempt to obtain a reachable PostgreSQL
	// instance (spawn or connect, selected by DATABASE_URL), retried up to
	// postgresMaxAttempts with postgresBackoff.
	obtainPostgres      func(ctx context.Context, cfg *config.Config) error
	postgresMaxAttempts int
	postgresBackoff     time.Duration
	// sleep waits for the backoff duration and returns early on ctx cancellation.
	sleep func(ctx context.Context, d time.Duration)

	// runMigrations executes database schema migrations on startup.
	runMigrations func(ctx context.Context, cfg *config.Config) error

	// newPool constructs the connection pool and repositories once PostgreSQL
	// is reachable and migrated.
	newPool func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error)

	// newJobSystem constructs the background job worker pool against the
	// shared connection pool.
	newJobSystem func(cfg *config.Config, logger *slog.Logger, pool pgPool) (jobRunner, error)

	// watchParent watches the Electron host parent process PID for termination.
	watchParent func(pid int) error

	userConfigDir func() (string, error)

	stderr io.Writer
}

// waitForPostgres calls obtain up to maxAttempts times, sleeping backoff
// between failed attempts, and returns nil on the first success or the
// last attempt's error once the budget is exhausted.
//
// It checks ctx.Err() before every attempt and after failed attempts,
// returning immediately if cancelled so a shutdown signal stops retry loops promptly.
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

// gracefulShutdown executes the shutdown sequence: stop accepting new
// connections, allow in-flight requests to complete within the configured
// grace period, stop background jobs, drain observability sweeps, and close
// the database connection pool.
func gracefulShutdown(cfg *config.Config, deps runDeps, srv shutdownableServer, redirectSrv *http.Server, pool pgPool, jobSystem jobRunner, eventReaper *observability.Reaper, logger *slog.Logger) int {
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithDeadline(context.Background(), deps.clock.Now().Add(cfg.ShutdownGracePeriod))
	defer cancel()

	// The HTTP redirect listener (if any) drains concurrently alongside the main server.
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
				// The grace period expired with requests still in flight. Force-close
				// remaining connections to cancel them cleanly.
				if closeErr := srv.Close(); closeErr != nil {
					logger.Error("force-close after shutdown timeout failed", "error", closeErr.Error())
				}
			} else {
				logger.Error("shutdown did not complete cleanly", "error", err.Error())
			}
		}
	}()

	shutdownWG.Wait()

	// Stop the job worker pool after the HTTP server stops accepting work and
	// before closing the shared pgxpool required for job state persistence.
	if jobSystem != nil {
		jobCtx, jobCancel := context.WithDeadline(context.Background(), deps.clock.Now().Add(cfg.ShutdownGracePeriod))
		if err := jobSystem.Shutdown(jobCtx); err != nil {
			logger.Warn("job worker pool did not stop within the grace period", "error", err.Error())
		}
		jobCancel()
	}

	// Stop the retention reaper before closing the pool, waiting for any
	// in-flight retention sweep to finish.
	if eventReaper != nil {
		eventReaper.Stop()
	}

	if pool != nil {
		pool.Close()
	}
	logger.Info("shutdown complete")
	return 0
}

// run executes the server lifecycle: load configuration, initialize logger,
// router, and listener, verify PostgreSQL reachability, run migrations,
// initialize repositories and background systems, and handle graceful shutdown.
func run(ctx context.Context, deps runDeps) int {
	cfg, err := deps.loadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(deps.stderr, "startup failed at step \"config\": %v\n", err)
		return 1
	}

	logger := deps.newLogger(cfg)
	logger.Info("startup step completed", "step", "config")
	logger.Info("startup step completed", "step", "logger")

	// The per-user data directory, used for certificate caches and keys.
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

	// Rate-limit keying: trust X-Forwarded-For only from these proxy
	// ranges, and collapse IPv6 clients to their /64 (#195).
	transporthttp.SetTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)

	// The unauthenticated health-probe rate limiter, with periodic eviction
	// to prevent unbounded map growth.
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
		// Announces bound ephemeral port to Electron host process.
		fmt.Printf("PORT=%d\n", tcpAddr.Port)
		boundPort = strconv.Itoa(tcpAddr.Port)
	}
	logger.Info("startup step completed", "step", "listen", "address", listener.Addr().String())

	// TLS configuration and optional redirect listener.
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
		// Path is omitted from logs for privacy; only explicit/default status is logged.
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
			// A shutdown signal terminated startup. Proceed with graceful shutdown.
			return gracefulShutdown(cfg, deps, srv, redirectSrv, nil, nil, nil, logger)
		}
		msg := err.Error()
		if cfg.DatabaseURL != "" {
			// Redact potential connection string / DSN details from log output.
			msg = "could not connect to the configured database"
		}
		logger.Error("startup failed", "step", "postgres", "error", msg)
		return 1
	}
	logger.Info("startup step completed", "step", "postgres")

	if err := deps.runMigrations(ctx, cfg); err != nil {
		if ctx.Err() != nil {
			return gracefulShutdown(cfg, deps, srv, redirectSrv, nil, nil, nil, logger)
		}
		msg := err.Error()
		if cfg.DatabaseURL != "" {
			msg = "could not run migrations against the configured database"
		}
		logger.Error("startup failed", "step", "migrate", "error", msg)
		return 1
	}
	logger.Info("startup step completed", "step", "migrate")

	// Held so gracefulShutdown can wait for an in-flight retention sweep to finish.
	var eventReaper *observability.Reaper

	pool, repos, err := deps.newPool(ctx, cfg)
	if err != nil {
		if ctx.Err() != nil {
			return gracefulShutdown(cfg, deps, srv, redirectSrv, nil, nil, nil, logger)
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
				Devices:        repos.pairedDevices,
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

	readerGrantSubkey, err := cryptoSvc.DeriveSubkey("reader-content-grant-v1")
	if err != nil {
		logger.Error("failed to derive reader content grant subkey", "error", err.Error())
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
			// Per-user MFA-verification throttle: 5 burst, then 1/min, evicted after 1 hour idle.
			mfaUserLimiter := auth.NewIPRateLimiter(rate.Every(time.Minute), 5, time.Hour)
			mfaUserLimiter.StartEviction(ctx)
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
				MFAUserLimiter:     mfaUserLimiter,
				MasterKey:          mfaSubkey,
				IDs:                idgen.New(),
				PairedDevices:      repos.pairedDevices,
				NetworkSettings:    repos.networkSettings,
				EnrolmentGrantJTIs: repos.enrolmentGrantJTIs,
				MFATicketJTIs:      repos.mfaTicketJTIs,
				EnrolmentSigner:    enrolmentSigner,
				ReaderGrants:       auth.NewReaderContentGrantSigner(readerGrantSubkey, "alexandryn"),
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
				Editions:       repos.editions,
				SyncStore:      repos.readingSync,
				Transactor:     repos.transactor,
				IDs:            idgen.New(),
				Now:            time.Now,
			})
		}
		if pgxPool, ok := pool.(*pgxpool.Pool); ok {
			poolRef.SetDBPool(pgxPool)
			// Feed live connection-pool stats to the diagnostics endpoint.
			if reg, ok := poolRef.GetMetricsRegistry(); ok {
				reg.SetPoolStatsProvider(poolStatsProvider(pgxPool))
			}
			eventStore := observability.NewEventStore(pgxPool, time.Now)
			poolRef.SetEventStore(eventStore)
			// System events retention reaper. Sweeps periodically to delete events past purge_at.
			eventReaper = observability.NewReaper(eventStore, time.Hour, logger)
			eventReaper.Start(ctx)
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
			if jsConcrete, ok := js.(*jobs.System); ok {
				poolRef.SetJobSystem(jsConcrete)
				poolRef.SetJobQueue(jsConcrete.Queue())
				// Feed live per-state job counts to the diagnostics endpoint.
				if reg, ok := poolRef.GetMetricsRegistry(); ok {
					reg.SetQueueDepthProvider(queueDepthProvider(jsConcrete.Queue().CountByState))
				}
				if repos != nil && repos.importCandidates != nil {
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
		return gracefulShutdown(cfg, deps, srv, redirectSrv, pool, jobSystem, eventReaper, logger)
	case err := <-serveErr:
		// The server stopped without a shutdown signal. Close background jobs
		// and the database pool before exiting.
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
