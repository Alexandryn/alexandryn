package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// validEnv sets default values for required environment variables (e.g.
// OPEN_LIBRARY_USER_AGENT) so individual tests can isolate and test specific keys.
func validEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OPEN_LIBRARY_USER_AGENT", "Alexandryn/dev (test)")
}

// Tests verifying configuration precedence:

func TestLoad_Precedence(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		setEnv  string
		want    func(*config.Config) any
		wantErr bool
	}{
		{
			name:   "LOG_LEVEL: no source uses the compiled default",
			key:    "LOG_LEVEL",
			setEnv: "",
			want:   func(c *config.Config) any { return c.LogLevel },
		},
		{
			name:   "LOG_LEVEL: environment overrides the default",
			key:    "LOG_LEVEL",
			setEnv: "debug",
			want:   func(c *config.Config) any { return c.LogLevel },
		},
		{
			name:   "SHUTDOWN_GRACE_PERIOD: environment overrides the default",
			key:    "SHUTDOWN_GRACE_PERIOD",
			setEnv: "30s",
			want:   func(c *config.Config) any { return c.ShutdownGracePeriod },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			if tc.setEnv != "" {
				t.Setenv(tc.key, tc.setEnv)
			}

			cfg, err := config.Load("", noFile, fakeUserConfigDir)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}

			got := tc.want(cfg)
			if tc.setEnv == "debug" && got != "debug" {
				t.Fatalf("LogLevel = %v, want %q (environment must override the default)", got, "debug")
			}
			if tc.setEnv == "30s" && got != 30*time.Second {
				t.Fatalf("ShutdownGracePeriod = %v, want 30s (environment must override the default)", got)
			}
		})
	}
}

func TestLoad_DefaultAppliesWhenNothingSet(t *testing.T) {
	validEnv(t)

	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want the compiled default %q", cfg.LogLevel, "info")
	}
	if cfg.ShutdownGracePeriod != 10*time.Second {
		t.Fatalf("ShutdownGracePeriod = %v, want the compiled default 10s", cfg.ShutdownGracePeriod)
	}
}

func TestLoad_EveryOptionalKeySetSimultaneouslyResolvesIndependently(t *testing.T) {
	validEnv(t)
	t.Setenv("DATABASE_URL", "postgres://fixture/proof")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("SHUTDOWN_GRACE_PERIOD", "5s")
	t.Setenv("DB_POOL_MAX_CONNS", "25")
	t.Setenv("HTTP_MAX_BODY_BYTES", "1048576")
	t.Setenv("HTTP_READ_TIMEOUT", "3s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "4s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "5s")

	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"DatabaseURL", cfg.DatabaseURL.Reveal(), "postgres://fixture/proof"},
		{"LogLevel", cfg.LogLevel, "warn"},
		{"ShutdownGracePeriod", cfg.ShutdownGracePeriod, 5 * time.Second},
		{"DBPoolMaxConns", cfg.DBPoolMaxConns, 25},
		{"HTTPMaxBodyBytes", cfg.HTTPMaxBodyBytes, int64(1048576)},
		{"HTTPReadTimeout", cfg.HTTPReadTimeout, 3 * time.Second},
		{"HTTPWriteTimeout", cfg.HTTPWriteTimeout, 4 * time.Second},
		{"HTTPIdleTimeout", cfg.HTTPIdleTimeout, 5 * time.Second},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v — one source incorrectly affected a different key", c.name, c.got, c.want)
		}
	}
}

// Tests verifying required and optional category behaviors:

func TestLoad_RequiredKeyMissingErrors(t *testing.T) {
	// OPEN_LIBRARY_USER_AGENT deliberately left unset.
	_, err := config.Load("", noFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error naming the missing required key")
	}
	if !strings.Contains(err.Error(), "OPEN_LIBRARY_USER_AGENT") {
		t.Fatalf("error %q doesn't name the missing key", err.Error())
	}
}

func TestLoad_OptionalWithDefaultKeyAbsentReturnsDefaultNotError(t *testing.T) {
	validEnv(t)
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (LOG_LEVEL is optional with a default)", err)
	}
	if cfg.LogLevel == "" {
		t.Fatal("LogLevel is empty, want the compiled default to have been applied")
	}
}

func TestLoad_DatabaseURLAbsentIsZeroValueNotError(t *testing.T) {
	validEnv(t)
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil — DATABASE_URL's absence is a meaningful signal, not a failure", err)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("DatabaseURL = %q, want the zero value when absent", cfg.DatabaseURL)
	}
}

// Table-driven tests for type and enum validation:

func TestLoad_TypeValidation(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{"LOG_LEVEL accepts debug", "LOG_LEVEL", "debug", false},
		{"LOG_LEVEL accepts info", "LOG_LEVEL", "info", false},
		{"LOG_LEVEL accepts warn", "LOG_LEVEL", "warn", false},
		{"LOG_LEVEL accepts error", "LOG_LEVEL", "error", false},
		{"LOG_LEVEL accepts mixed case", "LOG_LEVEL", "Debug", false},
		{"LOG_LEVEL accepts upper case", "LOG_LEVEL", "INFO", false},
		{"LOG_LEVEL rejects an unknown value", "LOG_LEVEL", "verbose", true},
		{"LOG_LEVEL rejects an unknown value even after lowercasing", "LOG_LEVEL", "VERBOSE", true},

		{"SHUTDOWN_GRACE_PERIOD accepts a parseable duration", "SHUTDOWN_GRACE_PERIOD", "15s", false},
		{"SHUTDOWN_GRACE_PERIOD rejects a non-parseable value", "SHUTDOWN_GRACE_PERIOD", "fifteen seconds", true},

		{"DB_POOL_MAX_CONNS accepts a parseable integer", "DB_POOL_MAX_CONNS", "10", false},
		{"DB_POOL_MAX_CONNS rejects a non-parseable value", "DB_POOL_MAX_CONNS", "many", true},

		{"HTTP_MAX_BODY_BYTES accepts a parseable integer", "HTTP_MAX_BODY_BYTES", "1024", false},
		{"HTTP_MAX_BODY_BYTES rejects a non-parseable value", "HTTP_MAX_BODY_BYTES", "1kb", true},

		{"HTTP_READ_TIMEOUT accepts a parseable duration", "HTTP_READ_TIMEOUT", "2s", false},
		{"HTTP_READ_TIMEOUT rejects a non-parseable value", "HTTP_READ_TIMEOUT", "soon", true},

		{"HTTP_WRITE_TIMEOUT accepts a parseable duration", "HTTP_WRITE_TIMEOUT", "2s", false},
		{"HTTP_WRITE_TIMEOUT rejects a non-parseable value", "HTTP_WRITE_TIMEOUT", "soon", true},

		{"HTTP_IDLE_TIMEOUT accepts a parseable duration", "HTTP_IDLE_TIMEOUT", "2s", false},
		{"HTTP_IDLE_TIMEOUT rejects a non-parseable value", "HTTP_IDLE_TIMEOUT", "soon", true},

		{"OPEN_LIBRARY_USER_AGENT accepts a non-empty string", "OPEN_LIBRARY_USER_AGENT", "Alexandryn/dev", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			t.Setenv(tc.key, tc.value)

			_, err := config.Load("", noFile, fakeUserConfigDir)
			if tc.wantErr && err == nil {
				t.Fatalf("Load() error = nil, want an error for %s=%q", tc.key, tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Load() error = %v, want nil for %s=%q", err, tc.key, tc.value)
			}
		})
	}
}

// Tests verifying that error messages identify the specific problematic key:

func TestLoad_ErrorContentNamesTheKey(t *testing.T) {
	cases := []struct {
		name         string
		key          string
		value        string
		wantSub      string
		skipValidEnv bool
	}{
		{
			name:         "missing required key",
			skipValidEnv: true,
			wantSub:      "OPEN_LIBRARY_USER_AGENT",
		},
		{
			name:    "type validation failure",
			key:     "LOG_LEVEL",
			value:   "verbose",
			wantSub: "LOG_LEVEL",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.skipValidEnv {
				validEnv(t)
			}
			if tc.key != "" {
				t.Setenv(tc.key, tc.value)
			}

			_, err := config.Load("", noFile, fakeUserConfigDir)
			if err == nil {
				t.Fatalf("Load() error = nil, want an error naming %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q doesn't name %q", err.Error(), tc.wantSub)
			}
			if strings.Contains(strings.ToLower(err.Error()), "invalid configuration") {
				t.Fatalf("error %q should name the specific failure", err.Error())
			}
		})
	}
}

// Tests for resilience against unrecognized keys and unstructured values:

func TestLoad_UnrelatedEnvironmentVariableIsIgnored(t *testing.T) {
	validEnv(t)
	t.Setenv("HTTP_REQUEST_TIMEOUT", "5s") // retired key name

	if _, err := config.Load("", noFile, fakeUserConfigDir); err != nil {
		t.Fatalf("Load() error = %v, want nil — an unrelated variable must have no effect", err)
	}
}

func TestLoad_DatabaseURLGarbageTextIsPassedThroughUnexamined(t *testing.T) {
	validEnv(t)
	t.Setenv("DATABASE_URL", "not a connection string at all")

	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.DatabaseURL != "not a connection string at all" {
		t.Fatalf("DatabaseURL = %q, want the raw value passed through unexamined", cfg.DatabaseURL)
	}
}

func TestLoad_DesktopParentPID(t *testing.T) {
	validEnv(t)

	// Unset -> defaults to 0 (optional with no default)
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.DesktopParentPID != 0 {
		t.Fatalf("DesktopParentPID = %d, want 0 when unset", cfg.DesktopParentPID)
	}

	// Set valid integer
	t.Setenv("DESKTOP_PARENT_PID", "12345")
	cfg, err = config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.DesktopParentPID != 12345 {
		t.Fatalf("DesktopParentPID = %d, want 12345", cfg.DesktopParentPID)
	}

	// Set invalid integer -> fails
	t.Setenv("DESKTOP_PARENT_PID", "not-a-pid")
	if _, err := config.Load("", noFile, fakeUserConfigDir); err == nil {
		t.Fatal("Load() error = nil, want error for non-integer DESKTOP_PARENT_PID")
	}
}
