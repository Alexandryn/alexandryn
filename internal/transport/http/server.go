package http

import (
	"net/http"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// NewServer constructs an *http.Server wired to cfg's HTTP timeout keys
// (backend-configuration.md FR-4) — the config-to-field wiring
// backend-http-transport.md FR-2 requires be a real, tested assignment,
// not left implicit.
func NewServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Handler:      handler,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}
}
