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

	"github.com/Alexandryn/alexandryn/internal/config"
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

// fakePool satisfies the pgPool interface (Ping + Close) run.go's step 6
// needs, without a real *pgxpool.Pool.
type fakePool struct {
	pingErr error
}

func (p *fakePool) Ping(context.Context) error { return p.pingErr }
func (p *fakePool) Close()                     {}

// recordingDeps builds a runDeps whose constructors each append their step
// name to order before returning, proving FR-1's step sequencing and that
// no later step's fake runs before an earlier one has returned. logger
// writes to a testutil.SpyHandler so tests can assert on logged content
// (redaction, step names) without parsing raw JSON.
func recordingDeps(t *testing.T, order *[]string) (runDeps, *testutil.SpyHandler) {
	t.Helper()
	fl := newFakeListener()
	t.Cleanup(func() { _ = fl.Close() })

	spy := testutil.NewSpyHandler()

	deps := runDeps{
		loadConfig: func() (*config.Config, error) {
			*order = append(*order, "config")
			return &config.Config{
				LogLevel:         "info",
				BindAddress:      "127.0.0.1:0",
				HTTPMaxBodyBytes: 1 << 20,
			}, nil
		},
		newLogger: func(cfg *config.Config) *slog.Logger {
			*order = append(*order, "logger")
			return slog.New(spy)
		},
		newRouter: func(cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef) http.Handler {
			*order = append(*order, "router")
			if poolRef == nil {
				t.Fatal("newRouter called with a nil poolRef")
			}
			if _, ok := poolRef.Get(); ok {
				t.Fatal("poolRef must start unset (FR-7) — the router was handed one already populated")
			}
			return http.NewServeMux()
		},
		listen: func(network, address string) (net.Listener, error) {
			*order = append(*order, "listen")
			return fl, nil
		},
		obtainPostgres: func(ctx context.Context, cfg *config.Config) error {
			*order = append(*order, "postgres")
			return nil
		},
		postgresMaxAttempts: 3,
		postgresBackoff:     0,
		sleep:               func(time.Duration) {},
		runMigrations: func(ctx context.Context, cfg *config.Config) error {
			*order = append(*order, "migrate")
			return nil
		},
		newPool: func(ctx context.Context, cfg *config.Config) (pgPool, error) {
			*order = append(*order, "pool")
			return &fakePool{}, nil
		},
		stderr: &bytes.Buffer{},
	}

	return deps, spy
}

func TestRun_ExecutesFR1StepsInOrder(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	run(ctx, deps)

	want := []string{"config", "logger", "router", "listen", "postgres", "migrate", "pool"}
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

// FR-7: the router is constructed with the pool reference before the
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

// FR-3 bounded retry: obtainPostgres fails twice, then succeeds — within
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
	deps.sleep = func(d time.Duration) { slept = append(slept, d) }

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	run(ctx, deps)

	if attempts != 3 {
		t.Fatalf("obtainPostgres was called %d times, want exactly 3 (2 failures + 1 success)", attempts)
	}
	if len(slept) != 2 {
		t.Fatalf("sleep was called %d times, want 2 (one backoff per failed attempt)", len(slept))
	}
	want := []string{"config", "logger", "router", "listen", "postgres", "migrate", "pool"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v — migrate/pool must run exactly once after the retry succeeds", order, want)
	}
}

// FR-3: a Postgres-connect fake that always fails exhausts the fixed
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
	deps.sleep = func(d time.Duration) { slept = append(slept, d) }

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

// FR-3 security amendment (review 0028): when DATABASE_URL is configured,
// the final startup-failure log line for the Postgres step must use a
// fixed generic message, never the underlying driver error's own text —
// pgx connection/parse errors can embed the DSN itself.
const fakeStartupDSNMarker = "postgres://startup-marker:s3cr3t@host/db"

func TestRun_PostgresFailureWithDatabaseURLNeverLeaksTheDSN(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:         "info",
			BindAddress:      "127.0.0.1:0",
			HTTPMaxBodyBytes: 1 << 20,
			DatabaseURL:      config.RedactedString(fakeStartupDSNMarker),
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

// Migration failure is an ordinary, single-attempt FR-3 failure, distinct
// from "unreachable" — pool must never be constructed.
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

// FR-7: once step 6 succeeds, the pool reference actually holds the
// constructed pool — the reference itself is the source of truth, not a
// separately tracked flag.
func TestRun_PoolReferencePopulatedAfterStep6(t *testing.T) {
	var order []string
	deps, _ := recordingDeps(t, &order)

	var capturedRef *transporthttp.PoolRef
	deps.newRouter = func(cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef) http.Handler {
		order = append(order, "router")
		capturedRef = poolRef
		return http.NewServeMux()
	}

	pool := &fakePool{}
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, error) {
		order = append(order, "pool")
		return pool, nil
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

func TestRun_PoolConstructionFailureStopsBeforeReady(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, error) {
		order = append(order, "pool")
		return nil, errors.New("pool: could not acquire connection")
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero on pool construction failure")
	}
	if !spy.Contains("pool") {
		t.Fatal("no log line names the failing step (pool)")
	}
}

// FR-3's DSN-redaction requirement isn't step-5-specific — it applies "at
// every log call it introduces" (spec's own Security considerations).
// pgxpool.ParseConfig's own error embeds the connection string (pgx
// redacts only the password, not host/user/dbname) when DATABASE_URL is
// malformed, so step 6's failure log needs the same guard step 5 has.
func TestRun_PoolConstructionFailureWithDatabaseURLNeverLeaksTheDSN(t *testing.T) {
	var order []string
	deps, spy := recordingDeps(t, &order)
	deps.loadConfig = func() (*config.Config, error) {
		order = append(order, "config")
		return &config.Config{
			LogLevel:         "info",
			BindAddress:      "127.0.0.1:0",
			HTTPMaxBodyBytes: 1 << 20,
			DatabaseURL:      config.RedactedString(fakeStartupDSNMarker),
		}, nil
	}
	deps.newPool = func(ctx context.Context, cfg *config.Config) (pgPool, error) {
		return nil, errors.New("cannot parse `" + fakeStartupDSNMarker + "`: invalid port")
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if spy.Contains(fakeStartupDSNMarker) {
		t.Fatal("pool-construction failure log leaked the DSN")
	}
	if !spy.Contains("could not construct the connection pool for the configured database") {
		t.Fatal("pool-construction failure log doesn't use a fixed generic message")
	}
}
