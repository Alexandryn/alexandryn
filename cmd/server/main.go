// Command server is Alexandryn's backend entry point.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/logging"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
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
		stderr:    os.Stderr,
	}))
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
