package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	"github.com/Alexandryn/alexandryn/internal/testutil"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// fakeListener satisfies net.Listener without opening a real socket — Accept
// blocks until closed, so http.Server.Serve just parks until the test tears
// it down via ctx cancellation.
type fakeListener struct {
	closed chan struct{}
}

func newFakeListener() *fakeListener { return &fakeListener{closed: make(chan struct{})} }

func (l *fakeListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, errors.New("fakeListener: closed")
}
func (l *fakeListener) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return nil
}
func (l *fakeListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4zero, Port: 0} }

// fakeServer satisfies shutdownableServer without opening a real socket.
// Serve blocks until Shutdown is called (or serveErr is set, for the
// "server crashed on its own" case); Shutdown records every context it was
// called with and appends "shutdown" to order, so tests can assert both
// call count/deadline and ordering relative to pool.Close. closeCalls
// counts Close invocations, so tests can assert the force-close-on-
// grace-period-timeout behavior.
type fakeServer struct {
	order        *[]string
	served       chan struct{}
	serveErr     error
	shutdownErr  error
	shutdownCtxs []context.Context
	closeCalls   int
	closeErr     error
}

func newFakeServer(order *[]string) *fakeServer {
	return &fakeServer{order: order, served: make(chan struct{})}
}

func (s *fakeServer) Serve(net.Listener) error {
	if s.serveErr != nil {
		return s.serveErr
	}
	<-s.served
	return http.ErrServerClosed
}

func (s *fakeServer) Shutdown(ctx context.Context) error {
	*s.order = append(*s.order, "shutdown")
	s.shutdownCtxs = append(s.shutdownCtxs, ctx)
	select {
	case <-s.served:
	default:
		close(s.served)
	}
	return s.shutdownErr
}

func (s *fakeServer) Close() error {
	s.closeCalls++
	return s.closeErr
}

// fakePool satisfies the pgPool interface (Ping + Close) without a real
// *pgxpool.Pool. When order is non-nil, Close appends "poolClose" to it,
// so tests can assert shutdown ordering.
type fakePool struct {
	pingErr error
	order   *[]string
}

func (p *fakePool) Ping(context.Context) error { return p.pingErr }
func (p *fakePool) Close() {
	if p.order != nil {
		*p.order = append(*p.order, "poolClose")
	}
}

// recordingDeps builds a runDeps whose constructors each append their step
// name to order before returning, verifying step sequencing and ensuring
// no later step runs before an earlier one completes. logger writes to a
// testutil.SpyHandler so tests can assert on logged content without parsing
// raw JSON.
func recordingDeps(t *testing.T, order *[]string) (runDeps, *testutil.SpyHandler) {
	t.Helper()
	fl := newFakeListener()
	t.Cleanup(func() { _ = fl.Close() })

	spy := testutil.NewSpyHandler()

	deps := runDeps{
		loadConfig: func() (*config.Config, error) {
			*order = append(*order, "config")
			return &config.Config{
				LogLevel:            "info",
				BindAddress:         "127.0.0.1:0",
				HTTPMaxBodyBytes:    1 << 20,
				ShutdownGracePeriod: 10 * time.Second,
			}, nil
		},
		newLogger: func(cfg *config.Config) *slog.Logger {
			*order = append(*order, "logger")
			return slog.New(spy)
		},
		newRouter: func(_ context.Context, cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef, _ *auth.IPRateLimiter) http.Handler {
			*order = append(*order, "router")
			if poolRef == nil {
				t.Fatal("newRouter called with a nil poolRef")
			}
			if _, ok := poolRef.Get(); ok {
				t.Fatal("poolRef must start unset — the router was handed one already populated")
			}
			return http.NewServeMux()
		},
		listen: func(network, address string) (net.Listener, error) {
			*order = append(*order, "listen")
			return fl, nil
		},
		newServer: func(cfg *config.Config, handler http.Handler) shutdownableServer {
			return newFakeServer(order)
		},
		clock: testutil.NewFakeClock(time.Unix(0, 0)),
		obtainPostgres: func(ctx context.Context, cfg *config.Config) error {
			*order = append(*order, "postgres")
			return nil
		},
		postgresMaxAttempts: 3,
		postgresBackoff:     0,
		sleep:               func(context.Context, time.Duration) {},
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			*order = append(*order, "migrate")
			return nil
		},
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
			*order = append(*order, "pool")
			return &fakePool{order: order}, nil, nil
		},
		stderr: &bytes.Buffer{},
	}

	return deps, spy
}

func TestRun_ExecutesStartupStepsInOrder(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	run(ctx, deps)

	want := []string{"config", "logger", "router", "listen", "postgres", "migrate", "pool", "shutdown", "poolClose"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("call order = %v, want %v", order, want)
		}
	}
}

func TestRun_ConfigFailureStopsBeforeAnyLaterStep(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return nil, errors.New("missing required configuration key OPEN_LIBRARY_USER_AGENT")
	}

	stderr := &bytes.Buffer{}
	deps.stderr = stderr

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on config failure")
	}
	if len(order) != 1 || order[0] != "config" {
		t.Fatalf("call order = %v, want only [config] — no later step may run after config fails", order)
	}
	if !strings.Contains(stderr.String(), "config") {
		t.Fatalf("stderr = %q, want it to name the failing step (config)", stderr.String())
	}
}

func TestRun_ListenFailureStopsBeforeServing(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)
	deps.listen = func(network, address string) (net.Listener, error) {
		order = append(order, "listen")
		return nil, errors.New("address already in use")
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on listener bind failure")
	}
	want := []string{"config", "logger", "router", "listen"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want exactly %v (postgres/migrate/pool must never run)", order, want)
	}
}

// The router is constructed with the pool reference before the
// listener is bound — by the time /healthz or /readyz could receive a
// request, the reference already exists and is unset.
func TestRun_PoolReferenceWiredBeforeListenerBinds(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	run(ctx, deps)

	routerIdx, listenIdx := -1, -1
	for i, step := range order {
		switch step {
		case "router":
			routerIdx = i
		case "listen":
			listenIdx = i
		}
	}
	if routerIdx == -1 || listenIdx == -1 || routerIdx > listenIdx {
		t.Fatalf("router must be constructed before the listener binds, got order %v", order)
	}
}

// Bounded retry: obtainPostgres fails twice, then succeeds — within
// budget, run proceeds to migrate/pool exactly once each.
func TestRun_PostgresRetrySucceedsWithinBudget(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	attempts := 0
	var slept []time.Duration
	deps.obtainPostgres = func(ctx context.Context, cfg *config.Config) error {
		attempts++
		if attempts < 3 {
			return errors.New("connection refused")
		}
		order = append(order, "postgres")
		return nil
	}
	deps.postgresMaxAttempts = 5
	deps.postgresBackoff = 10 * time.Millisecond
	deps.sleep = func(ctx context.Context, d time.Duration) { slept = append(slept, d) }

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	if attempts != 3 {
		t.Fatalf("obtainPostgres was called %d times, want exactly 3 (2 failures + 1 success)", attempts)
	}
	if len(slept) != 2 {
		t.Fatalf("sleep was called %d times, want 2 (one backoff per failed attempt)", len(slept))
	}
	want := []string{"config", "logger", "router", "listen", "postgres", "migrate", "pool", "shutdown", "poolClose"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v — migrate/pool must run exactly once after the retry succeeds", order, want)
	}
}

// A Postgres-connect fake that always fails exhausts the fixed
// retry budget, makes exactly that many attempts (never more, never
// fewer), and produces an ordinary non-zero-exit startup failure —
// migrate and pool must never run.
func TestRun_PostgresRetryExhaustsBudgetThenFails(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)

	attempts := 0
	deps.obtainPostgres = func(ctx context.Context, cfg *config.Config) error {
		attempts++
		return errors.New("connection refused")
	}
	deps.postgresMaxAttempts = 4
	deps.postgresBackoff = time.Millisecond
	var slept []time.Duration
	deps.sleep = func(ctx context.Context, d time.Duration) { slept = append(slept, d) }

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero once the retry budget is exhausted")
	}
	if attempts != 4 {
		t.Fatalf("obtainPostgres was called %d times, want exactly the budget of 4", attempts)
	}
	if len(slept) != 3 {
		t.Fatalf("sleep was called %d times, want budget-1 = 3 (no backoff after the final failed attempt)", len(slept))
	}
	want := []string{"config", "logger", "router", "listen"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want exactly %v — migrate/pool must never run", order, want)
	}
	if !spy.Contains("postgres") {
		t.Fatal("no log line names the failing step (postgres)")
	}
}

// When DATABASE_URL is configured, the final startup-failure log line for
// the Postgres step must use a fixed generic message, never the underlying
// driver error's text, to prevent leaking DSN connection details.
const fakeStartupDSNMarker = "postgres://startup-marker:s3cr3t@host/db"

func TestRun_PostgresFailureWithDatabaseURLNeverLeaksTheDSN(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:            "info",
			BindAddress:         "127.0.0.1:0",
			HTTPMaxBodyBytes:    1 << 20,
			ShutdownGracePeriod: 10 * time.Second,
			DatabaseURL:         config.RedactedString(fakeStartupDSNMarker),
		}, nil
	}
	deps.obtainPostgres = func(ctx context.Context, cfg *config.Config) error {
		return errors.New("dial tcp " + fakeStartupDSNMarker + ": connection refused")
	}
	deps.postgresMaxAttempts = 1
	deps.postgresBackoff = 0

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if spy.Contains(fakeStartupDSNMarker) {
		t.Fatal("startup failure log leaked the DSN")
	}
	if !spy.Contains("could not connect to the configured database") {
		t.Fatal("startup failure log doesn't use the fixed generic message")
	}
}

// Converse of the above: with no DATABASE_URL configured (the spawn path),
// the real underlying error is safe to log as-is — no connection string to
// redact — and doing so is what makes a spawn failure diagnosable.
func TestRun_PostgresFailureWithoutDatabaseURLLogsTheRealError(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.obtainPostgres = func(ctx context.Context, cfg *config.Config) error {
		return errors.New("spawning a managed PostgreSQL instance is not implemented yet")
	}
	deps.postgresMaxAttempts = 1
	deps.postgresBackoff = 0

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !spy.Contains("spawning a managed PostgreSQL instance is not implemented yet") {
		t.Fatal("startup failure log doesn't surface the real spawn error, want it unredacted (no DATABASE_URL in play)")
	}
}

// Migration failure is a startup failure distinct from connectivity errors;
// pool must never be constructed.
func TestRun_MigrationFailureStopsBeforePool(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.runMigrations = func(ctx context.Context, cfg *config.Config) error {
		order = append(order, "migrate")
		return errors.New("migration failed: relation \"foo\" already exists")
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on migration failure")
	}
	want := []string{"config", "logger", "router", "listen", "postgres", "migrate"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want exactly %v — pool must never be constructed", order, want)
	}
	if !spy.Contains("migrate") {
		t.Fatal("no log line names the failing step (migrate)")
	}
}

// Once pool construction succeeds, the pool reference holds the constructed pool.
func TestRun_PoolReferencePopulatedAfterStep6(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	var capturedRef *transporthttp.PoolRef
	deps.newRouter = func(_ context.Context, cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef, _ *auth.IPRateLimiter) http.Handler {
		order = append(order, "router")
		capturedRef = poolRef
		return http.NewServeMux()
	}

	pool := &fakePool{}
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
		order = append(order, "pool")
		return pool, nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	got, ok := capturedRef.Get()
	if !ok {
		t.Fatal("pool reference is still unset after a successful startup")
	}
	if got != pgPool(pool) {
		t.Fatal("pool reference doesn't hold the pool step 6 constructed")
	}
}

func TestRun_SourceRepositoriesAndCryptoPopulatedAfterStep6(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)
	deps.userConfigDir = func() (string, error) { return t.TempDir(), nil }

	var capturedRef *transporthttp.PoolRef
	deps.newRouter = func(_ context.Context, cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef, _ *auth.IPRateLimiter) http.Handler {
		order = append(order, "router")
		capturedRef = poolRef
		return http.NewServeMux()
	}

	pool := &fakePool{}
	repos := &repositories{
		sourceRecords: postgres.NewSourceRecordRepository(nil),
		sourceRemoval: domain.NewSourceRemovalService(nil, nil, nil),
	}
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
		order = append(order, "pool")
		return pool, repos, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	if _, ok := capturedRef.GetSourceRecordRepository(); !ok {
		t.Fatal("source record repository is not set on poolRef")
	}
	if _, ok := capturedRef.GetSourceRemovalService(); !ok {
		t.Fatal("source removal service is not set on poolRef")
	}
	sc, ok := capturedRef.GetSourceCrypto()
	if !ok || sc.Encryptor == nil || sc.Codec == nil {
		t.Fatal("source crypto is not set on poolRef")
	}
}

func TestRun_PoolConstructionFailureStopsBeforeReady(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
		order = append(order, "pool")
		return nil, nil, errors.New("pool: could not acquire connection")
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on pool construction failure")
	}
	if !spy.Contains("pool") {
		t.Fatal("no log line names the failing step (pool)")
	}
}

// DSN redaction applies to connection pool construction failures as well as
// reachability checks: pgxpool.ParseConfig's error can embed the connection string
// when DATABASE_URL is malformed.
func TestRun_PoolConstructionFailureWithDatabaseURLNeverLeaksTheDSN(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:            "info",
			BindAddress:         "127.0.0.1:0",
			HTTPMaxBodyBytes:    1 << 20,
			ShutdownGracePeriod: 10 * time.Second,
			DatabaseURL:         config.RedactedString(fakeStartupDSNMarker),
		}, nil
	}
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
		order = append(order, "pool")
		return nil, nil, errors.New("parse error: invalid DSN " + fakeStartupDSNMarker)
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on pool construction failure")
	}
	if spy.Contains(fakeStartupDSNMarker) {
		t.Fatalf("startup failure log leaked the DSN marker %q", fakeStartupDSNMarker)
	}
	if !spy.Contains("could not construct the connection pool for the configured database") {
		t.Fatal("pool-construction failure log doesn't use a fixed generic message")
	}
}

// Sending the shutdown signal invokes http.Server.Shutdown with a context
// whose deadline is clock.Now() + configured grace period.
func TestRun_Shutdown_UsesConfiguredGracePeriodDeadline(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := testutil.NewFakeClock(fixed)
	deps.clock = fc

	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:            "info",
			BindAddress:         "127.0.0.1:0",
			HTTPMaxBodyBytes:    1 << 20,
			ShutdownGracePeriod: 7 * time.Second,
		}, nil
	}

	var srv *fakeServer
	deps.newServer = func(cfg *config.Config, handler http.Handler) shutdownableServer {
		srv = newFakeServer(&order)
		return srv
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	if len(srv.shutdownCtxs) != 1 {
		t.Fatalf("Shutdown was called %d times, want exactly 1", len(srv.shutdownCtxs))
	}
	gotDeadline, ok := srv.shutdownCtxs[0].Deadline()
	if !ok {
		t.Fatal("the context passed to Shutdown has no deadline")
	}
	wantDeadline := fixed.Add(7 * time.Second)
	if !gotDeadline.Equal(wantDeadline) {
		t.Fatalf("Shutdown deadline = %v, want %v (clock.Now() + ShutdownGracePeriod)", gotDeadline, wantDeadline)
	}
}

// The pool closes only after Shutdown returns, never before or concurrently.
func TestRun_Shutdown_PoolClosesStrictlyAfterShutdownReturns(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	shutdownIdx, closeIdx := -1, -1
	for i, step := range order {
		switch step {
		case "shutdown":
			shutdownIdx = i
		case "poolClose":
			closeIdx = i
		}
	}
	if shutdownIdx == -1 {
		t.Fatal("Shutdown was never called on a clean shutdown signal")
	}
	if closeIdx == -1 {
		t.Fatal("the pool was never closed on a clean shutdown signal")
	}
	if closeIdx <= shutdownIdx {
		t.Fatalf("pool closed at index %d, Shutdown called at index %d — pool must close strictly after Shutdown returns", closeIdx, shutdownIdx)
	}
}

// When Shutdown returns context.DeadlineExceeded (the grace period expired
// with requests still in flight), the process still proceeds to close the pool
// and exits cleanly.
func TestRun_Shutdown_GracePeriodExpiryStillClosesPoolAndExitsZero(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)
	var srv *fakeServer
	deps.newServer = func(cfg *config.Config, handler http.Handler) shutdownableServer {
		srv = newFakeServer(&order)
		srv.shutdownErr = context.DeadlineExceeded
		return srv
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	code := run(ctx, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 — grace-period expiry is expected, not a failure", code)
	}
	shutdownIdx, closeIdx := -1, -1
	for i, step := range order {
		switch step {
		case "shutdown":
			shutdownIdx = i
		case "poolClose":
			closeIdx = i
		}
	}
	if shutdownIdx == -1 || closeIdx == -1 || closeIdx <= shutdownIdx {
		t.Fatalf("call order = %v, want shutdown then poolClose even when Shutdown times out", order)
	}
	// Shutdown alone only waits for active connections; a grace-period timeout
	// must force-close what's left via Close.
	if srv.closeCalls != 1 {
		t.Fatalf("Close was called %d times, want exactly 1 after a grace-period timeout", srv.closeCalls)
	}
}

// The pool must close regardless of why the process is exiting: if the
// server stops on its own (never via a shutdown signal), no Shutdown is
// called, but the pool must still close before the process exits.
func TestRun_UnexpectedServerCrash_ClosesPoolWithoutCallingShutdown(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.newServer = func(cfg *config.Config, handler http.Handler) shutdownableServer {
		s := newFakeServer(&order)
		s.serveErr = errors.New("listener closed unexpectedly")
		return s
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on an unexpected server crash")
	}
	for _, step := range order {
		if step == "shutdown" {
			t.Fatalf("Shutdown must not be called on an unexpected crash, got order %v", order)
		}
	}
	found := false
	for _, step := range order {
		if step == "poolClose" {
			found = true
		}
	}
	if !found {
		t.Fatalf("pool must still be closed on an unexpected server crash, got order %v", order)
	}
	if !spy.Contains("http server stopped unexpectedly") {
		t.Fatal("no log line reports the unexpected server crash")
	}
}

// A shutdown signal arriving while waitForPostgres's retry loop is
// still running must be treated as a shutdown, halting retries promptly
// and executing the shutdown sequence.
func TestRun_ShutdownSignalDuringPostgresRetry_CallsShutdownNotOrdinaryFailure(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	deps.obtainPostgres = func(ctx context.Context, cfg *config.Config) error {
		attempts++
		if attempts == 1 {
			cancel()
		}
		return errors.New("connection refused")
	}
	deps.postgresMaxAttempts = 30
	deps.postgresBackoff = time.Hour // would hang the test if the shutdown check didn't interrupt it
	deps.sleep = func(ctx context.Context, d time.Duration) {
		<-ctx.Done() // the real sleepOrDone's shape: returns on ctx cancellation
	}

	done := make(chan int, 1)
	go func() { done <- run(ctx, deps) }()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 — a shutdown signal is not a startup failure", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run() did not return — the shutdown signal was not observed during the retry loop")
	}

	if attempts > 2 {
		t.Fatalf("obtainPostgres was called %d times, want at most 2 — the retry loop must stop immediately on cancellation, not keep retrying", attempts)
	}
	shutdownCalled := false
	for _, step := range order {
		if step == "shutdown" {
			shutdownCalled = true
		}
	}
	if !shutdownCalled {
		t.Fatalf("Shutdown was never called, call order = %v — a mid-startup shutdown signal must still attempt Shutdown", order)
	}
}

// Same gap, at the migration step: a shutdown signal arriving while
// runMigrations is in flight must route to Shutdown, not be logged as an
// ordinary migration failure.
func TestRun_ShutdownSignalDuringMigration_CallsShutdownNotOrdinaryFailure(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)

	ctx, cancel := context.WithCancel(context.Background())
	deps.runMigrations = func(ctx context.Context, cfg *config.Config) error {
		order = append(order, "migrate")
		cancel()
		return errors.New("migration failed: context canceled")
	}

	code := run(ctx, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 — a shutdown signal is not a startup failure", code)
	}
	if spy.Contains("could not run migrations") {
		t.Fatal("a mid-startup shutdown signal was logged as an ordinary migration failure")
	}
	shutdownCalled := false
	for _, step := range order {
		if step == "shutdown" {
			shutdownCalled = true
		}
	}
	if !shutdownCalled {
		t.Fatalf("Shutdown was never called, call order = %v", order)
	}
}

// Same gap, at the pool-construction step.
func TestRun_ShutdownSignalDuringPoolConstruction_CallsShutdownNotOrdinaryFailure(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)

	ctx, cancel := context.WithCancel(context.Background())
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, *repositories, error) {
		order = append(order, "pool")
		cancel()
		return nil, nil, errors.New("pool: context canceled")
	}

	code := run(ctx, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 — a shutdown signal is not a startup failure", code)
	}
	if spy.Contains("could not construct the connection pool") {
		t.Fatal("a mid-startup shutdown signal was logged as an ordinary pool-construction failure")
	}
	shutdownCalled := false
	for _, step := range order {
		if step == "shutdown" {
			shutdownCalled = true
		}
	}
	if !shutdownCalled {
		t.Fatalf("Shutdown was never called, call order = %v", order)
	}
}

// Defense in depth (Checkpoint F review, LOW finding): the migrate step's
// failure log needs the same DSN-redaction guard the postgres/pool steps
// have, even though today internal/persistence/postgres.RunMigrations
// already pre-sanitizes connection-class failures — this is the
// regression guard for if that ever stops being true.
func TestRun_MigrationFailureWithDatabaseURLNeverLeaksTheDSN(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:            "info",
			BindAddress:         "127.0.0.1:0",
			HTTPMaxBodyBytes:    1 << 20,
			ShutdownGracePeriod: 10 * time.Second,
			DatabaseURL:         config.RedactedString(fakeStartupDSNMarker),
		}, nil
	}
	deps.runMigrations = func(ctx context.Context, cfg *config.Config) error {
		return errors.New("could not connect: " + fakeStartupDSNMarker)
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if spy.Contains(fakeStartupDSNMarker) {
		t.Fatal("migration failure log leaked the DSN")
	}
	if !spy.Contains("could not run migrations against the configured database") {
		t.Fatal("migration failure log doesn't use a fixed generic message")
	}
}

func TestRun_ParentWatch_InvokedWhenDesktopParentPIDSet(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:            "info",
			BindAddress:         "127.0.0.1:0",
			HTTPMaxBodyBytes:    1 << 20,
			ShutdownGracePeriod: 10 * time.Second,
			DesktopParentPID:    9999,
		}, nil
	}
	var watchedPID int
	deps.watchParent = func(pid int) error {
		watchedPID = pid
		order = append(order, "parentwatch")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	code := run(ctx, deps)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if watchedPID != 9999 {
		t.Fatalf("watchedPID = %d, want 9999", watchedPID)
	}
	if !spy.Contains("parentwatch") {
		t.Fatal("expected parentwatch step to be logged")
	}
}

func TestRun_ParentWatch_FailureHaltsStartup(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:            "info",
			BindAddress:         "127.0.0.1:0",
			HTTPMaxBodyBytes:    1 << 20,
			ShutdownGracePeriod: 10 * time.Second,
			DesktopParentPID:    9999,
		}, nil
	}
	deps.watchParent = func(pid int) error {
		return errors.New("cannot watch parent")
	}

	code := run(context.Background(), deps)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero when watchParent fails")
	}
	if !spy.Contains("cannot watch parent") {
		t.Fatal("expected watchParent error to be logged")
	}
}

// fakeJobRunner records Start and Shutdown calls against the shared order
// slice so tests can assert startup and shutdown sequencing.
type fakeJobRunner struct {
	order        *[]string
	shutdownErr  error
	shutdownCtxs []context.Context
}

func (j *fakeJobRunner) Start(context.Context) {
	*j.order = append(*j.order, "jobStart")
}

func (j *fakeJobRunner) Shutdown(ctx context.Context) error {
	*j.order = append(*j.order, "jobShutdown")
	j.shutdownCtxs = append(j.shutdownCtxs, ctx)
	return j.shutdownErr
}

func indexOf(order []string, step string) int {
	for i, s := range order {
		if s == step {
			return i
		}
	}
	return -1
}

// The job worker pool starts after the pool is constructed and stops between
// the HTTP server's Shutdown and the pool's Close.
func TestRun_JobWorkerPoolStartsAfterPoolAndStopsBeforePoolClose(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)
	jr := &fakeJobRunner{order: &order}
	deps.newJobSystem = func(*config.Config, *slog.Logger, pgPool) (jobRunner, error) { return jr, nil }

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	poolIdx := indexOf(order, "pool")
	startIdx := indexOf(order, "jobStart")
	shutdownIdx := indexOf(order, "shutdown")
	jobShutdownIdx := indexOf(order, "jobShutdown")
	closeIdx := indexOf(order, "poolClose")

	if poolIdx == -1 || startIdx == -1 || shutdownIdx == -1 || jobShutdownIdx == -1 || closeIdx == -1 {
		t.Fatalf("missing a step in order = %v", order)
	}
	if startIdx < poolIdx {
		t.Fatalf("job pool started before the connection pool: %v", order)
	}
	if shutdownIdx >= jobShutdownIdx || jobShutdownIdx >= closeIdx {
		t.Fatalf("shutdown order = %v; want shutdown < jobShutdown < poolClose", order)
	}
}

// The job worker pool's shutdown context uses the configured grace period
// as its deadline bound, computed off the injected clock.
func TestRun_JobWorkerPoolShutdownUsesGracePeriodDeadline(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	deps.clock = testutil.NewFakeClock(fixed)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:            "info",
			BindAddress:         "127.0.0.1:0",
			HTTPMaxBodyBytes:    1 << 20,
			ShutdownGracePeriod: 9 * time.Second,
		}, nil
	}
	jr := &fakeJobRunner{order: &order}
	deps.newJobSystem = func(*config.Config, *slog.Logger, pgPool) (jobRunner, error) { return jr, nil }

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	if len(jr.shutdownCtxs) != 1 {
		t.Fatalf("job Shutdown called %d times, want 1", len(jr.shutdownCtxs))
	}
	deadline, ok := jr.shutdownCtxs[0].Deadline()
	if !ok || !deadline.Equal(fixed.Add(9*time.Second)) {
		t.Fatalf("job Shutdown deadline = %v (ok=%v), want %v", deadline, ok, fixed.Add(9*time.Second))
	}
}

// A job-subsystem construction failure fails startup cleanly:
// logged, non-zero exit, and pool closed.
func TestRun_JobSystemConstructionFailureExitsNonZero(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.newJobSystem = func(*config.Config, *slog.Logger, pgPool) (jobRunner, error) {
		return nil, errors.New("job pool wiring is broken")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	code := run(ctx, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if indexOf(order, "poolClose") == -1 {
		t.Fatalf("pool was not closed after the job-subsystem failure: %v", order)
	}
	if !spy.Contains("startup failed") {
		t.Fatal("expected a startup-failed log line for the jobs step")
	}
}
