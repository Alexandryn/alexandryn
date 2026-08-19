package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// runDeps carries every one of run's constructor dependencies as fields,
// never package-level state (backend-service-lifecycle.md FR-2). main
// populates it with the real config/logging/transport packages; tests
// substitute fakes that record call order.
type runDeps struct {
	loadConfig func() (*config.Config, error)
	newLogger  func(cfg *config.Config) *slog.Logger
	newRouter  func(cfg *config.Config, logger *slog.Logger, poolRef *transporthttp.PoolRef) http.Handler
	listen     func(network, address string) (net.Listener, error)
	stderr     io.Writer
}

// run executes backend-service-lifecycle.md FR-1's startup sequence, steps
// 1-4: load and validate config, construct the logger, construct the
// router and middleware chain with an atomically-held, still-empty pool
// reference wired to /healthz and /readyz (FR-7), then bind the listener
// and start serving — the process is now "alive."
//
// Steps 5-7 (connect PostgreSQL, run migrations, construct the pool and
// populate the reference, report Ready) and graceful shutdown (FR-4/5/6)
// are not implemented yet — this is where they slot in, between the
// listener bind below and the shutdown wait at the end.
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

	srv := transporthttp.NewServer(cfg, router)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()

	// T18 inserts FR-1 steps 5-6 here (connect PostgreSQL, run migrations,
	// construct the pool, populate poolRef) before this wait. T19 replaces
	// this bare wait with FR-4/5/6's graceful shutdown sequence.
	select {
	case <-ctx.Done():
		return 0
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err.Error())
			return 1
		}
		return 0
	}
}
