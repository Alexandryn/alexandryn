package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// T20's Integration/Concurrency layers, per backend-service-lifecycle.md's
// own test plan: a real net.Listener bound by the actual *http.Server
// (never httptest.Server, which doesn't exercise Shutdown's real
// accept-loop-closure path), driven through the real run() function.
// These specific cases don't need a real PostgreSQL — obtainPostgres/
// runMigrations/newPool are faked for speed and determinism — so they run
// as plain tests (backend-test-harness.md FR-1: unit tests must not
// require Postgres, Docker, or any external service). The two cases that
// do need real PostgreSQL live in run_integration_test.go under the
// integration build tag.

// realListenDeps returns a deps.listen that wraps a real net.Listen and
// publishes the bound listener's address on addrCh the moment it's
// available — the only way a test can learn the OS-assigned port
// BindAddress "127.0.0.1:0" produces.
func realListenDeps(addrCh chan<- string) func(network, address string) (net.Listener, error) {
	return func(network, address string) (net.Listener, error) {
		ln, err := net.Listen(network, address)
		if err == nil {
			addrCh <- ln.Addr().String()
		}
		return ln, err
	}
}

func realServerDeps() func(cfg *config.Config, handler http.Handler) shutdownableServer {
	return func(cfg *config.Config, handler http.Handler) shutdownableServer {
		return transporthttp.NewServer(cfg, handler)
	}
}

func quietLogger(*config.Config) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForAddr(t *testing.T, addrCh <-chan string, timeout time.Duration) string {
	t.Helper()
	select {
	case addr := <-addrCh:
		return addr
	case <-time.After(timeout):
		t.Fatal("listener never bound — no address was published")
		return ""
	}
}

func waitForExit(t *testing.T, exitCh <-chan int, timeout time.Duration) int {
	t.Helper()
	select {
	case code := <-exitCh:
		return code
	case <-time.After(timeout):
		t.Fatal("run() never returned")
		return -1
	}
}

// waitForStatus polls url until it returns wantStatus or timeout elapses.
func waitForStatus(t *testing.T, url string, wantStatus int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	var lastStatus int
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(10 * time.Millisecond)
			continue
		}
		lastStatus = resp.StatusCode
		_ = resp.Body.Close()
		if lastStatus == wantStatus {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("GET %s never reached status %d within %v (last status %d, last error %v)", url, wantStatus, timeout, lastStatus, lastErr)
}

func baseRealServerConfig() *config.Config {
	return &config.Config{
		LogLevel:            "error",
		BindAddress:         "127.0.0.1:0",
		HTTPMaxBodyBytes:    1 << 20,
		HTTPReadTimeout:     5 * time.Second,
		HTTPWriteTimeout:    5 * time.Second,
		HTTPIdleTimeout:     5 * time.Second,
		ShutdownGracePeriod: 2 * time.Second,
	}
}

// FR-7's central claim, proven for real: /healthz answers 200 the instant
// the listener is bound, while /readyz still answers 503, for as long as
// step 5 (obtaining PostgreSQL) hasn't completed — over a real bound
// listener and real HTTP, not httptest.
func TestIntegration_AliveBeforeReadyWindow(t *testing.T) {
	cfg := baseRealServerConfig()

	unblock := make(chan struct{})
	addrCh := make(chan string, 1)

	deps := runDeps{
		loadConfig: func() (*config.Config, error) { return cfg, nil },
		newLogger:  quietLogger,
		newRouter:  newProductionRouter,
		listen:     realListenDeps(addrCh),
		newServer:  realServerDeps(),
		clock:      realClock{},
		obtainPostgres: func(ctx context.Context, cfg *config.Config) error {
			select {
			case <-unblock:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		postgresMaxAttempts: 1,
		postgresBackoff:     0,
		sleep:               sleepOrDone,
		runMigrations:       func(context.Context, *config.Config) error { return nil },
		newPool:             func(context.Context, *config.Config) (pgPool, *repositories, error) { return &fakePool{}, nil, nil },
		stderr:              io.Discard,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exitCh := make(chan int, 1)
	go func() { exitCh <- run(ctx, deps) }()

	addr := waitForAddr(t, addrCh, 2*time.Second)
	base := "http://" + addr

	waitForStatus(t, base+"/healthz", http.StatusOK, 2*time.Second)

	resp, err := http.Get(base + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("/readyz status = %d, want 503 — PostgreSQL hasn't been obtained yet, the process is alive but not ready", resp.StatusCode)
	}

	close(unblock)
	waitForStatus(t, base+"/readyz", http.StatusOK, 2*time.Second)

	cancel()
	code := waitForExit(t, exitCh, 3*time.Second)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

// FR-3, run through the real config.Load: a genuinely invalid config
// (OPEN_LIBRARY_USER_AGENT unset) makes run() exit non-zero before any
// other step runs — PostgreSQL is never touched.
func TestIntegration_InvalidConfigNeverTouchesPostgres(t *testing.T) {
	t.Setenv("OPEN_LIBRARY_USER_AGENT", "")

	postgresTouched := false

	deps := runDeps{
		loadConfig: func() (*config.Config, error) {
			return config.Load("", func(string) ([]byte, error) { return nil, fmt.Errorf("no config file in this test") }, func() (string, error) { return "", fmt.Errorf("no user config dir in this test") })
		},
		obtainPostgres: func(context.Context, *config.Config) error {
			postgresTouched = true
			return nil
		},
		stderr: io.Discard,
	}

	code := run(context.Background(), deps)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero — OPEN_LIBRARY_USER_AGENT is unset, config.Load must fail")
	}
	if postgresTouched {
		t.Fatal("obtainPostgres was called despite invalid config — PostgreSQL must never be touched when startup fails at step 1")
	}
}

// The Concurrency layer named as this plan's top risk: a real listener, a
// real *http.Server, several deliberately slow concurrent handlers, and a
// shutdown signal sent while all of them are in flight. Requests shorter
// than the grace period must complete normally; the one exceeding it must
// be cleanly cancelled, never left to hang past the grace period and
// never given a truncated body. A second, immediate cancellation must not
// panic or double-close the pool.
func TestConcurrency_ShutdownUnderLoad(t *testing.T) {
	const (
		numUnder    = 5
		gracePeriod = 300 * time.Millisecond
		underDelay  = 180 * time.Millisecond // > half the grace period, < the full period
		overDelay   = 900 * time.Millisecond // deliberately exceeds the grace period
	)

	cfg := baseRealServerConfig()
	cfg.ShutdownGracePeriod = gracePeriod

	var started sync.WaitGroup
	started.Add(numUnder + 1)

	mux := http.NewServeMux()

	mux.HandleFunc("/slow-under", func(w http.ResponseWriter, r *http.Request) {
		started.Done()
		select {
		case <-time.After(underDelay):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case <-r.Context().Done():
			// An under-grace request must never observe cancellation.
		}
	})
	mux.HandleFunc("/slow-over", func(w http.ResponseWriter, r *http.Request) {
		started.Done()
		select {
		case <-time.After(overDelay):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case <-r.Context().Done():
			// Expected: cancelled by the grace-period force-close.
		}
	})

	addrCh := make(chan string, 1)
	var closeCalls int
	var closeMu sync.Mutex

	deps := runDeps{
		loadConfig: func() (*config.Config, error) { return cfg, nil },
		newLogger:  quietLogger,
		newRouter: func(_ context.Context, cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef, _ *auth.IPRateLimiter) http.Handler {
			mux.Handle("/healthz", transporthttp.Healthz(poolRef))
			return mux
		},
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      func(context.Context, *config.Config) error { return nil },
		postgresMaxAttempts: 1,
		postgresBackoff:     0,
		sleep:               sleepOrDone,
		runMigrations:       func(context.Context, *config.Config) error { return nil },
		newPool: func(context.Context, *config.Config) (pgPool, *repositories, error) {
			return &countingClosePool{onClose: func() {
				closeMu.Lock()
				closeCalls++
				closeMu.Unlock()
			}}, nil, nil
		},
		stderr: io.Discard,
	}

	ctx, cancel := context.WithCancel(context.Background())
	exitCh := make(chan int, 1)
	go func() { exitCh <- run(ctx, deps) }()

	addr := waitForAddr(t, addrCh, 2*time.Second)
	base := "http://" + addr
	waitForStatus(t, base+"/healthz", http.StatusOK, 2*time.Second)

	client := &http.Client{Timeout: 5 * time.Second}
	type reqResult struct {
		name   string
		status int
		err    error
	}
	results := make(chan reqResult, numUnder+1)

	var wg sync.WaitGroup
	for i := 0; i < numUnder; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := client.Get(base + "/slow-under")
			if err != nil {
				results <- reqResult{name: fmt.Sprintf("under-%d", i), err: err}
				return
			}
			defer func() { _ = resp.Body.Close() }()
			_, _ = io.Copy(io.Discard, resp.Body)
			results <- reqResult{name: fmt.Sprintf("under-%d", i), status: resp.StatusCode}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := client.Get(base + "/slow-over")
		if err != nil {
			results <- reqResult{name: "over", err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		results <- reqResult{name: "over", status: resp.StatusCode}
	}()

	// Wait until every handler has actually started before signalling
	// shutdown — the scenario is "requests already in flight," not
	// "requests that happen to race the signal."
	startedCh := make(chan struct{})
	go func() { started.Wait(); close(startedCh) }()
	select {
	case <-startedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("not all handlers started in time")
	}

	// The signal, and an immediate second one — context.CancelFunc is
	// idempotent by the stdlib's own contract, so this is the faithful
	// shape "double SIGTERM" takes through this exact codebase's own
	// cancellation mechanism (main.go's real signal.NotifyContext also
	// only ever delivers one cancellation to this code; a second real
	// SIGTERM reverts to the OS default disposition). Asserting this
	// doesn't panic and doesn't double-close the pool is the full extent
	// of what's specified for the double-signal case.
	cancel()
	cancel()

	code := waitForExit(t, exitCh, gracePeriod+2*time.Second)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	wg.Wait()
	close(results)

	gotUnder, gotOver := 0, false
	for r := range results {
		if r.name == "over" {
			gotOver = true
			if r.err == nil {
				t.Fatalf("over-grace request got status %d with no error, want a clean cancellation/reset since its delay (%v) exceeds the grace period (%v)", r.status, overDelay, gracePeriod)
			}
			continue
		}
		gotUnder++
		if r.err != nil {
			t.Fatalf("%s: unexpected error %v, want a normal 200 (its delay %v is within the grace period %v)", r.name, r.err, underDelay, gracePeriod)
		}
		if r.status != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", r.name, r.status)
		}
	}
	if gotUnder != numUnder {
		t.Fatalf("got %d under-grace results, want %d", gotUnder, numUnder)
	}
	if !gotOver {
		t.Fatal("never got a result for the over-grace request")
	}

	closeMu.Lock()
	defer closeMu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("pool Close was called %d times, want exactly 1 — the double signal must not double-close the pool", closeCalls)
	}
}

// countingClosePool is a pgPool whose Close always succeeds and reports
// itself via onClose, for the double-signal/double-close assertion.
type countingClosePool struct {
	onClose func()
}

func (p *countingClosePool) Ping(context.Context) error { return nil }
func (p *countingClosePool) Close()                     { p.onClose() }
