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

// recordingDeps builds a runDeps whose four constructors each append their
// step name to order before returning, proving FR-1's step sequencing and
// that no later step's fake runs before an earlier one has returned.
func recordingDeps(t *testing.T, order *[]string) runDeps {
	t.Helper()
	fl := newFakeListener()
	t.Cleanup(func() { _ = fl.Close() })

	return runDeps{
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
			return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
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
		stderr: &bytes.Buffer{},
	}
}

func TestRun_ExecutesFR1StepsInOrder(t *testing.T) {
	var order []string
	deps := recordingDeps(t, &order)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	run(ctx, deps)

	want := []string{"config", "logger", "router", "listen"}
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
	deps := recordingDeps(t, &order)
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
	deps := recordingDeps(t, &order)
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
		t.Fatalf("call order = %v, want exactly %v (serve must never start)", order, want)
	}
}

// FR-7: the router is constructed with the pool reference before the
// listener is bound — by the time /healthz or /readyz could receive a
// request, the reference already exists and is unset.
func TestRun_PoolReferenceWiredBeforeListenerBinds(t *testing.T) {
	var order []string
	deps := recordingDeps(t, &order)

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
