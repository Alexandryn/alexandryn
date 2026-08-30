// Package config resolves Alexandryn's backend configuration: one
// compiled-in default, then a config file, then an environment variable,
// each overriding the previous source for that one key independently
// (backend-configuration.md FR-2). This is the one package allowed to
// read an environment variable or the config file directly
// (architecture-backend.md FR-1) — everything else receives a *Config,
// constructed once at startup.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config is Alexandryn's fully resolved, validated backend configuration.
// Load either returns one of these with every field valid, or an error —
// never a partially populated value.
type Config struct {
	// DatabaseURL is optional with no compiled default: its absence is
	// itself a meaningful signal backend-persistence.md FR-5 uses to
	// choose between connecting to it and spawning a bundled instance
	// (backend-configuration.md FR-3's third category). Its type carries
	// FR-7's redaction — call .Reveal() to get the real value, never log
	// or print this field directly by any other means.
	DatabaseURL RedactedString

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

	// BindAddress is classified per FR-8/ADR 0017: loopback or a private
	// range is always legal; a publicly routable address is legal only
	// with TLSCertFile/TLSKeyFile both present and valid.
	BindAddress string

	// TLSCertFile and TLSKeyFile are required only when BindAddress
	// resolves to a publicly routable address; otherwise unread
	// (backend-configuration.md FR-4).
	TLSCertFile string
	TLSKeyFile  string

	// DesktopParentPID is optional: when set by the Electron desktop host
	// (architecture-desktop-host.md FR-8, desktop-host-process-model.md FR-6),
	// the server watches this PID for termination and self-exits if the parent dies.
	DesktopParentPID int
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
		apply:    func(cfg *Config, v any) { cfg.DatabaseURL = RedactedString(v.(string)) },
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
	{
		key:      "BIND_ADDRESS",
		category: categoryOptionalDefault,
		parse:    parseHostPort,
		apply:    func(cfg *Config, v any) { cfg.BindAddress = v.(string) },
	},
	{
		key:      "TLS_CERT_FILE",
		category: categoryOptionalNoDefault,
		parse:    parseString,
		apply:    func(cfg *Config, v any) { cfg.TLSCertFile = v.(string) },
	},
	{
		key:      "TLS_KEY_FILE",
		category: categoryOptionalNoDefault,
		parse:    parseString,
		apply:    func(cfg *Config, v any) { cfg.TLSKeyFile = v.(string) },
	},
	{
		key:      "DESKTOP_PARENT_PID",
		category: categoryOptionalNoDefault,
		parse:    parseInt,
		apply:    func(cfg *Config, v any) { cfg.DesktopParentPID = v.(int) },
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
	"BIND_ADDRESS":          "127.0.0.1:0",
}

// Load resolves and validates every configuration key and returns a
// fully populated Config, or the first error encountered — never a
// partially populated value (backend-configuration.md FR-1).
//
// configPath is the operator-supplied --config value, or "" if the flag
// was absent (in which case userConfigDir locates the well-known
// fallback location, per FR-5). readFile and userConfigDir are injected
// so Load never touches the real filesystem in a test — production
// callers pass os.ReadFile and os.UserConfigDir directly.
func Load(configPath string, readFile func(path string) ([]byte, error), userConfigDir func() (string, error)) (*Config, error) {
	fileValues, err := loadFileValues(configPath, readFile, userConfigDir)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	for _, f := range fields {
		raw, present := os.LookupEnv(f.key)
		if !present || raw == "" {
			if fv, ok := fileValues[strings.ToLower(f.key)]; ok {
				raw, present = fv, true
			}
		}

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

	if err := validateBindAddress(cfg, readFile); err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseHostPort(raw string) (any, error) {
	if _, _, err := net.SplitHostPort(raw); err != nil {
		return nil, fmt.Errorf("must be host:port: %w", err)
	}
	return raw, nil
}

// loadFileValues resolves the config file's path (explicit --config, or
// the well-known fallback) and, if a file is present, parses it into a
// key-to-string map every field's own parse function can consume the
// same way it consumes an environment value (ADR 0019).
func loadFileValues(configPath string, readFile func(string) ([]byte, error), userConfigDir func() (string, error)) (map[string]string, error) {
	path := configPath
	explicit := configPath != ""
	if !explicit {
		dir, err := userConfigDir()
		if err != nil {
			return nil, fmt.Errorf("could not resolve the default configuration directory: %w", err)
		}
		path = filepath.Join(dir, "alexandryn", "config.toml")
	}

	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if explicit {
				return nil, fmt.Errorf("config file %s does not exist", path)
			}
			// No --config flag and nothing at the fallback location:
			// not an error (FR-5) — every key resolves from the
			// environment or its default.
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("could not read config file %s: %w", path, err)
	}

	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		// Deliberately never include the underlying parser error's own
		// text: it can echo the offending line's raw content verbatim,
		// which is exactly what FR-6 forbids when that line holds
		// DATABASE_URL or any future sensitive key. Position (line,
		// column) is safe; the parser's own formatted message is not.
		var decodeErr *toml.DecodeError
		if errors.As(err, &decodeErr) {
			row, col := decodeErr.Position()
			return nil, fmt.Errorf("config file %s contains invalid TOML syntax at line %d, column %d", path, row, col)
		}
		return nil, fmt.Errorf("config file %s contains invalid TOML syntax", path)
	}

	values := make(map[string]string, len(raw))
	for k, v := range raw {
		values[k] = tomlValueToString(v)
	}
	return values, nil
}

func tomlValueToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
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
