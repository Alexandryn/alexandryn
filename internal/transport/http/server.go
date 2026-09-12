package http

import (
	"net/http"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// NewServer constructs an *http.Server wired to cfg's HTTP timeout keys,
// ensuring the configured timeouts are explicitly applied to the server instance.
func NewServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Handler:      handler,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}
}
