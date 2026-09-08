// Package config resolves Alexandryn's backend configuration: one
// compiled-in default, then a config file, then an environment variable,
// each overriding the previous source for that one key independently
// (backend-configuration.md FR-2). This is the one package allowed to
// read an environment variable or the config file directly
// (architecture-backend.md FR-1) — everything else receives a *Config,
// constructed once at startup.
package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
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

	// Phase 13 network keys (backend-configuration.md FR-4 amendment,
	// ADR 0028). ACMEEnabled selects the certificate provisioning source
	// for a public bind — ACME issuance vs. the static TLSCertFile/
	// TLSKeyFile pair; it is not a security toggle (both branches fail
	// closed). ACMECacheDir empty means "the acme/ subdirectory of the
	// per-user data directory" — resolved at startup where that path is
	// known, not here.
	ACMEEnabled  bool
	ACMEDomain   string
	ACMEEmail    string
	ACMECacheDir string

	// CORSAllowedOrigins is empty by default: no cross-origin request is
	// honoured and the same-origin SPA is unaffected. Each entry is a
	// well-formed scheme://host[:port] with no path.
	CORSAllowedOrigins []string

	// DevicePairingSecret is an optional operator-set extra factor on
	// POST /api/v1/network/pair/initiate (ADR 0028 §6). Redacted — never
	// log or print this field directly; call .Reveal() for the real
	// value.
	DevicePairingSecret RedactedString

	// SourceAllowPrivateAddresses permits outbound source (OPDS) requests
	// to reach loopback and RFC 1918 / IPv6-ULA private addresses. Off by
	// default: a source base URL is attacker-chosen input (§4), and the
	// outbound client blocks non-public targets and defends DNS rebinding.
	// A self-hoster whose OPDS server runs on the same machine or their
	// own LAN sets this true. Link-local (cloud metadata), CGNAT, and
	// multicast stay blocked regardless.
	SourceAllowPrivateAddresses bool

	// DesktopParentPID is optional: when set by the Electron desktop host
	// (architecture-desktop-host.md FR-8, desktop-host-process-model.md FR-6),
	// the server watches this PID for termination and self-exits if the parent dies.
	DesktopParentPID int

	// tlsCert is the validated serving certificate for an in-process TLS
	// bind (a public bind, or a private bind with the opt-in
	// TLS_CERT_FILE/TLS_KEY_FILE). nil means "serve plaintext" — a
	// loopback/private bind with no cert. Populated by validateBindAddress
	// during Load; read via TLSCertificate.
	tlsCert *tls.Certificate

	// reachability and tlsMode are set by validateBindAddress from
	// BIND_ADDRESS's class plus the certificate/ACME state (ADR 0028 §1).
	// reachability: "loopback" | "private" | "public".
	// tlsMode:      "none" (plaintext, upstream TLS may terminate) |
	//               "static" (in-process, file cert) |
	//               "acme" (in-process, autocert-issued).
	reachability string
	tlsMode      string

	// namedBindHost is BIND_ADDRESS's host when it is a DNS name on a
	// public bind (empty for a bare-IP bind or any non-public class) —
	// set once by validateBindAddress's own classification, so a caller
	// (the :80 redirect's canonical host) never re-derives "is this a
	// name" with a second copy of the same check.
	namedBindHost string
}

// NamedBindHost returns BIND_ADDRESS's host when Reachability() is
// "public" and the host is a DNS name (not a bare IP literal); empty
// otherwise.
func (c *Config) NamedBindHost() string { return c.namedBindHost }

// TLSCertificate returns the validated in-process-TLS serving certificate,
// or nil when the bind is plaintext (loopback/private with no cert) or
// when certificates come from ACME (TLSMode == "acme"). When non-nil,
// cmd/server MUST wrap the listener in TLS.
func (c *Config) TLSCertificate() *tls.Certificate { return c.tlsCert }

// Reachability reports whether BIND_ADDRESS is "loopback", "private", or
// "public" (ADR 0028 §1). "public" is Mode A — in-process TLS mandatory
// and a :80 HTTP->HTTPS redirect listener runs.
func (c *Config) Reachability() string { return c.reachability }

// TLSMode reports how TLS terminates: "none" (plaintext on this
// listener), "static" (in-process, TLS_CERT_FILE/TLS_KEY_FILE), or "acme"
// (in-process, autocert). backend-network-api.md FR-4's /network/status
// reports it.
func (c *Config) TLSMode() string { return c.tlsMode }

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
		key:      "SOURCE_ALLOW_PRIVATE_ADDRESSES",
		category: categoryOptionalDefault,
		parse:    parseBool,
		apply:    func(cfg *Config, v any) { cfg.SourceAllowPrivateAddresses = v.(bool) },
	},
	{
		key:      "DESKTOP_PARENT_PID",
		category: categoryOptionalNoDefault,
		parse:    parseInt,
		apply:    func(cfg *Config, v any) { cfg.DesktopParentPID = v.(int) },
	},
	{
		key:      "ACME_ENABLED",
		category: categoryOptionalDefault,
		parse:    parseBool,
		apply:    func(cfg *Config, v any) { cfg.ACMEEnabled = v.(bool) },
	},
	{
		key:      "ACME_DOMAIN",
		category: categoryOptionalNoDefault,
		parse:    parseString,
		apply:    func(cfg *Config, v any) { cfg.ACMEDomain = v.(string) },
	},
	{
		key:      "ACME_EMAIL",
		category: categoryOptionalNoDefault,
		parse:    parseString,
		apply:    func(cfg *Config, v any) { cfg.ACMEEmail = v.(string) },
	},
	{
		key:      "ACME_CACHE_DIR",
		category: categoryOptionalNoDefault,
		parse:    parseString,
		apply:    func(cfg *Config, v any) { cfg.ACMECacheDir = v.(string) },
	},
	{
		key:      "CORS_ALLOWED_ORIGINS",
		category: categoryOptionalNoDefault,
		parse:    parseOriginList,
		apply:    func(cfg *Config, v any) { cfg.CORSAllowedOrigins = v.([]string) },
	},
	{
		key:      "DEVICE_PAIRING_SECRET",
		category: categoryOptionalNoDefault,
		parse:    func(raw string) (any, error) { return raw, nil },
		apply:    func(cfg *Config, v any) { cfg.DevicePairingSecret = RedactedString(v.(string)) },
	},
}

// defaults holds each categoryOptionalDefault key's compiled default,
// applied when no source provides a value. DB_POOL_MAX_CONNS and the
// three HTTP_*_TIMEOUT/HTTP_MAX_BODY_BYTES values are provisional
// placeholders — see the Config field comments above.
var defaults = map[string]any{
	"LOG_LEVEL":                      "info",
	"SHUTDOWN_GRACE_PERIOD":          10 * time.Second,
	"DB_POOL_MAX_CONNS":              10,
	"HTTP_MAX_BODY_BYTES":            int64(10 << 20),
	"HTTP_READ_TIMEOUT":              15 * time.Second,
	"HTTP_WRITE_TIMEOUT":             15 * time.Second,
	"HTTP_IDLE_TIMEOUT":              60 * time.Second,
	"BIND_ADDRESS":                   "127.0.0.1:0",
	"ACME_ENABLED":                   false,
	"SOURCE_ALLOW_PRIVATE_ADDRESSES": false,
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

// parseBool accepts the common truthy/falsey spellings strconv.ParseBool
// handles (1/t/T/TRUE/true/True and the 0/f/... negatives). An
// unrecognised value is an error naming what was expected.
func parseBool(raw string) (any, error) {
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("must be a boolean (true/false/1/0): got %q", raw)
	}
	return b, nil
}

// parseOriginList splits a comma-separated CORS_ALLOWED_ORIGINS value,
// trims each entry, validates it is a bare scheme://host[:port] with an
// http/https scheme, a host, and no path/query/fragment (ADR 0028 §4:
// CORS matching is exact string equality, so a malformed entry could
// never match and is rejected loudly instead), and normalizes it to the
// serialized-origin form the browser actually sends (RFC 6454 §6.1):
// host lower-cased, and the scheme's default port (:443 for https, :80
// for http) dropped. Without this, the reverse-proxy config ADR 0028 §4
// describes — an operator pasting https://host:443 straight from a proxy
// file — would sit in the list as an entry no Origin header can match.
// Normalization only ever tightens an eventual exact match, never loosens
// it. (url.Parse already lower-cases the scheme.)
func parseOriginList(raw string) (any, error) {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		u, err := url.Parse(entry)
		if err != nil {
			return nil, fmt.Errorf("entry %q is not a valid origin: %w", entry, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("entry %q must use the http or https scheme", entry)
		}
		if u.Hostname() == "" {
			return nil, fmt.Errorf("entry %q has no host", entry)
		}
		if u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("entry %q must be a bare scheme://host[:port] with no path", entry)
		}
		host := strings.ToLower(u.Hostname())
		if strings.Contains(host, ":") {
			// An IPv6 literal — u.Hostname() strips the brackets a
			// serialized origin keeps ("http://[::1]", RFC 6454 §6.1).
			host = "[" + host + "]"
		}
		if port := u.Port(); port != "" && !isDefaultPort(u.Scheme, port) {
			host += ":" + port
		}
		out = append(out, u.Scheme+"://"+host)
	}
	return out, nil
}

// isDefaultPort reports whether port is the scheme's default (dropped
// from a serialized origin per RFC 6454 §6.1).
func isDefaultPort(scheme, port string) bool {
	return (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
}
