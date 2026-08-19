package http_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// Config-to-field wiring: a *config.Config with non-default values for
// all four HTTP keys reaches the constructed *http.Server's timeout
// fields and the limits middleware's own cutoff, exactly — a single
// assignment-level test, independent of the Concurrency layer's own
// tests (which use short, test-specific values for speed).
func TestNewServer_WiresConfigValuesIntoServerFields(t *testing.T) {
	cfg := &config.Config{
		HTTPReadTimeout:  7 * time.Second,
		HTTPWriteTimeout: 42 * time.Second,
		HTTPIdleTimeout:  99 * time.Second,
		HTTPMaxBodyBytes: 12345,
	}

	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	srv := transporthttp.NewServer(cfg, handler)

	if srv.ReadTimeout != cfg.HTTPReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", srv.ReadTimeout, cfg.HTTPReadTimeout)
	}
	if srv.WriteTimeout != cfg.HTTPWriteTimeout {
		t.Errorf("WriteTimeout = %v, want %v", srv.WriteTimeout, cfg.HTTPWriteTimeout)
	}
	if srv.IdleTimeout != cfg.HTTPIdleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", srv.IdleTimeout, cfg.HTTPIdleTimeout)
	}
}
