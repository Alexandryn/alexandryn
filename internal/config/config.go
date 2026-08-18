// Package config resolves Alexandryn's backend configuration: one
// compiled-in default, then a config file (added in a later task), then
// an environment variable, each overriding the previous source for that
// one key independently (backend-configuration.md FR-2). This is the one
// package allowed to read an environment variable directly
// (architecture-backend.md FR-1) — everything else receives a *Config,
// constructed once at startup.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is Alexandryn's fully resolved, validated backend configuration.
// Load either returns one of these with every field valid, or an error —
// never a partially populated value.
type Config struct {
	// DatabaseURL is optional with no compiled default: its absence is
	// itself a meaningful signal backend-persistence.md FR-5 uses to
	// choose between connecting to it and spawning a bundled instance
	// (backend-configuration.md FR-3's third category).
	DatabaseURL string

	// LogLevel is one of "debug", "info", "warn", "error", matched
	// case-insensitively and stored lowercased.
	LogLevel string

	ShutdownGracePeriod time.Duration

	// DBPoolMaxConns' and the four fields below's real default values are
	// backend-persistence.md and backend-http-transport.md's to fix
	// (backend-configuration.md FR-4's table); this package only reserves
	// the key and a provisional default, both replaced when those specs
	// land (T8/T13 of tasks/plan.md).
	DBPoolMaxConns int

	HTTPMaxBodyBytes int64
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout  time.Duration

	// OpenLibraryUserAgent has no compiled default: a placeholder value
	// would misidentify this client to Open Library, which FR-3's
	// required category exists to prevent.
	OpenLibraryUserAgent string
}

type category int

const (
	// categoryRequired: no compiled default; absence from every source
	// is a validation error.
	categoryRequired category = iota
	// categoryOptionalDefault: a value exists even if no source provides
	// one.
	categoryOptionalDefault
	// categoryOptionalNoDefault: absence is itself a meaningful signal,
	// checked explicitly by the caller — never used as a "usually fine
	// to omit" escape hatch (backend-configuration.md FR-3).
	categoryOptionalNoDefault
)

// fieldSpec resolves one configuration key: parse validates and converts
// a present, non-empty raw value, and apply assigns the parsed value into
// cfg. Category governs what happens when no source provides a value.
type fieldSpec struct {
	key      string
	category category
	parse    func(raw string) (any, error)
	apply    func(cfg *Config, v any)
}

var fields = []fieldSpec{
	{
		key:      "DATABASE_URL",
		category: categoryOptionalNoDefault,
		parse:    func(raw string) (any, error) { return raw, nil },
		apply:    func(cfg *Config, v any) { cfg.DatabaseURL = v.(string) },
	},
	{
		key:      "LOG_LEVEL",
		category: categoryOptionalDefault,
		parse:    parseLogLevel,
		apply:    func(cfg *Config, v any) { cfg.LogLevel = v.(string) },
	},
	{
		key:      "SHUTDOWN_GRACE_PERIOD",
		category: categoryOptionalDefault,
		parse:    parseDuration,
		apply:    func(cfg *Config, v any) { cfg.ShutdownGracePeriod = v.(time.Duration) },
	},
	{
		key:      "DB_POOL_MAX_CONNS",
		category: categoryOptionalDefault,
		parse:    parseInt,
		apply:    func(cfg *Config, v any) { cfg.DBPoolMaxConns = v.(int) },
	},
	{
		key:      "HTTP_MAX_BODY_BYTES",
		category: categoryOptionalDefault,
		parse:    parseInt64,
		apply:    func(cfg *Config, v any) { cfg.HTTPMaxBodyBytes = v.(int64) },
	},
	{
		key:      "HTTP_READ_TIMEOUT",
		category: categoryOptionalDefault,
		parse:    parseDuration,
		apply:    func(cfg *Config, v any) { cfg.HTTPReadTimeout = v.(time.Duration) },
	},
	{
		key:      "HTTP_WRITE_TIMEOUT",
		category: categoryOptionalDefault,
		parse:    parseDuration,
		apply:    func(cfg *Config, v any) { cfg.HTTPWriteTimeout = v.(time.Duration) },
	},
	{
		key:      "HTTP_IDLE_TIMEOUT",
		category: categoryOptionalDefault,
		parse:    parseDuration,
		apply:    func(cfg *Config, v any) { cfg.HTTPIdleTimeout = v.(time.Duration) },
	},
	{
		key:      "OPEN_LIBRARY_USER_AGENT",
		category: categoryRequired,
		parse:    parseString,
		apply:    func(cfg *Config, v any) { cfg.OpenLibraryUserAgent = v.(string) },
	},
}

// defaults holds each categoryOptionalDefault key's compiled default,
// applied when no source provides a value. DB_POOL_MAX_CONNS and the
// three HTTP_*_TIMEOUT/HTTP_MAX_BODY_BYTES values are provisional
// placeholders — see the Config field comments above.
var defaults = map[string]any{
	"LOG_LEVEL":             "info",
	"SHUTDOWN_GRACE_PERIOD": 10 * time.Second,
	"DB_POOL_MAX_CONNS":     10,
	"HTTP_MAX_BODY_BYTES":   int64(10 << 20),
	"HTTP_READ_TIMEOUT":     15 * time.Second,
	"HTTP_WRITE_TIMEOUT":    15 * time.Second,
	"HTTP_IDLE_TIMEOUT":     60 * time.Second,
}

// Load resolves and validates every configuration key and returns a
// fully populated Config, or the first error encountered — never a
// partially populated value (backend-configuration.md FR-1).
func Load() (*Config, error) {
	cfg := &Config{}

	for _, f := range fields {
		raw, present := os.LookupEnv(f.key)
		if !present || raw == "" {
			switch f.category {
			case categoryRequired:
				return nil, fmt.Errorf("missing required configuration key %s", f.key)
			case categoryOptionalDefault:
				f.apply(cfg, defaults[f.key])
			case categoryOptionalNoDefault:
				// Leave the field at its zero value; absence is itself
				// a valid signal for this key.
			}
			continue
		}

		v, err := f.parse(raw)
		if err != nil {
			return nil, fmt.Errorf("value for %s is invalid: %w", f.key, err)
		}
		f.apply(cfg, v)
	}

	return cfg, nil
}

func parseLogLevel(raw string) (any, error) {
	lower := strings.ToLower(raw)
	switch lower {
	case "debug", "info", "warn", "error":
		return lower, nil
	default:
		return nil, fmt.Errorf("must be one of debug, info, warn, error (got %q)", raw)
	}
}

func parseDuration(raw string) (any, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return nil, fmt.Errorf("must be a valid duration: %w", err)
	}
	return d, nil
}

func parseInt(raw string) (any, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("must be a valid integer: %w", err)
	}
	return n, nil
}

func parseInt64(raw string) (any, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("must be a valid integer: %w", err)
	}
	return n, nil
}

func parseString(raw string) (any, error) {
	return raw, nil
}
