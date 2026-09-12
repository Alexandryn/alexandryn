package config_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// noFile simulates "nothing at this path" for any path — the shared
// no-config-file-anywhere fixture most tests in this package want.
func noFile(string) ([]byte, error) { return nil, fs.ErrNotExist }

// fakeUserConfigDir stands in for os.UserConfigDir in tests, returning a mock
// path recognized by mapReadFile.
func fakeUserConfigDir() (string, error) { return "/fake/home/.config", nil }

func mapReadFile(files map[string][]byte) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		data, ok := files[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return data, nil
	}
}

// Tests verifying configuration file resolution:

func TestLoad_ExplicitConfigPathLoadsWhenFileExists(t *testing.T) {
	validEnv(t)
	files := map[string][]byte{
		"/explicit/config.toml": []byte(`log_level = "warn"`),
	}
	cfg, err := config.Load("/explicit/config.toml", mapReadFile(files), fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q, want %q from the config file", cfg.LogLevel, "warn")
	}
}

func TestLoad_ExplicitConfigPathErrorsWhenFileMissing(t *testing.T) {
	_, err := config.Load("/explicit/does-not-exist.toml", noFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error — an explicitly wrong --config path is a real failure")
	}
	if !strings.Contains(err.Error(), "/explicit/does-not-exist.toml") {
		t.Fatalf("error %q doesn't name the missing path", err.Error())
	}
}

func TestLoad_NoConfigFlagAndNoFallbackFileIsNotAnError(t *testing.T) {
	validEnv(t)
	cfg, err := config.Load("", noFile, fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil — an absent fallback file is not itself an error", err)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want the compiled default", cfg.LogLevel)
	}
}

func TestLoad_NoConfigFlagLoadsFromTheFallbackLocation(t *testing.T) {
	validEnv(t)
	fallbackPath := filepath.Join("/fake/home/.config", "alexandryn", "config.toml")
	files := map[string][]byte{
		fallbackPath: []byte(`log_level = "debug"`),
	}
	cfg, err := config.Load("", mapReadFile(files), fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q from the fallback config file", cfg.LogLevel, "debug")
	}
}

// Tests verifying configuration precedence and file-sourced keys:

func TestLoad_FileSourceOnlySetsTheKey(t *testing.T) {
	validEnv(t)
	files := map[string][]byte{
		"/cfg.toml": []byte(`shutdown_grace_period = "20s"`),
	}
	cfg, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.ShutdownGracePeriod.String() != "20s" {
		t.Fatalf("ShutdownGracePeriod = %v, want 20s from the file", cfg.ShutdownGracePeriod)
	}
}

func TestLoad_EnvironmentOverridesFile(t *testing.T) {
	validEnv(t)
	files := map[string][]byte{
		"/cfg.toml": []byte(`log_level = "warn"`),
	}
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q — environment must override the file", cfg.LogLevel, "debug")
	}
}

// Tests verifying error handling for configuration file failures:

func TestLoad_InvalidTOMLSyntaxErrorsNamingTheFileNotTheContent(t *testing.T) {
	files := map[string][]byte{
		"/cfg.toml": []byte("database_url = \"unterminated string\ndatabase_url_leaked_secret_marker"),
	}
	_, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want a TOML syntax error")
	}
	if !strings.Contains(err.Error(), "/cfg.toml") {
		t.Fatalf("error %q doesn't name the file", err.Error())
	}
	if strings.Contains(err.Error(), "database_url_leaked_secret_marker") {
		t.Fatalf("error %q echoes the offending line's raw content — raw secrets must not leak in error strings", err.Error())
	}
}

func TestLoad_ConfigFilePresentButEmptyIsNotAnError(t *testing.T) {
	validEnv(t)
	files := map[string][]byte{
		"/cfg.toml": []byte(""),
	}
	cfg, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil — an empty file is not the same as a missing required key", err)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want the compiled default", cfg.LogLevel)
	}
}

func TestLoad_UnrecognizedKeyInFileIsIgnored(t *testing.T) {
	validEnv(t)
	files := map[string][]byte{
		"/cfg.toml": []byte(`custom_key = "value"` + "\n" + `log_level = "warn"`),
	}
	cfg, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil — an unrecognized key must be ignored, not rejected", err)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q, want %q — the recognized key beside the unrecognized one must still resolve", cfg.LogLevel, "warn")
	}
}

func TestLoad_DuplicateKeyInFileErrors(t *testing.T) {
	files := map[string][]byte{
		"/cfg.toml": []byte("log_level = \"warn\"\nlog_level = \"debug\"\n"),
	}
	_, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error — TOML forbids duplicate keys")
	}
}

func TestLoad_ConfigPathIsADirectory(t *testing.T) {
	dir := t.TempDir()
	readFile := func(path string) ([]byte, error) {
		if path == dir {
			return os.ReadFile(dir) // the real os.ReadFile's own is-a-directory error
		}
		return nil, fs.ErrNotExist
	}

	_, err := config.Load(dir, readFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error — a directory is not a valid config file")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("error %q doesn't name the path", err.Error())
	}
}

func TestLoad_ConfigPathNoReadPermission(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission denial is unreliable to test")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`log_level = "warn"`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Errorf("cleanup Chmod: %v", err)
		}
	})

	_, err := config.Load(path, os.ReadFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error — the file exists but cannot be read")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error %q doesn't name the path", err.Error())
	}
}

func TestLoad_UserConfigDirErrorPropagates(t *testing.T) {
	validEnv(t)
	failingUserConfigDir := func() (string, error) {
		return "", errors.New("no home directory")
	}

	_, err := config.Load("", noFile, failingUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error when the fallback directory can't be resolved")
	}
}

func TestLoad_FileValueOfTheWrongTOMLTypeStillProducesAClearError(t *testing.T) {
	cases := []struct {
		name string
		toml string
	}{
		{"a boolean where a string enum is expected", `log_level = true`},
		{"a float where a duration is expected", `shutdown_grace_period = 3.5`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			files := map[string][]byte{"/cfg.toml": []byte(tc.toml)}

			_, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
			if err == nil {
				t.Fatal("Load() error = nil, want a validation error — the file's own type doesn't match the key's declared type")
			}
		})
	}
}

func TestLoad_FileValuesWithNativeTOMLTypesAreConverted(t *testing.T) {
	validEnv(t)
	files := map[string][]byte{
		"/cfg.toml": []byte("db_pool_max_conns = 25\nhttp_max_body_bytes = 2048\n"),
	}

	cfg, err := config.Load("/cfg.toml", mapReadFile(files), fakeUserConfigDir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.DBPoolMaxConns != 25 {
		t.Fatalf("DBPoolMaxConns = %d, want 25 (from an unquoted TOML integer)", cfg.DBPoolMaxConns)
	}
	if cfg.HTTPMaxBodyBytes != 2048 {
		t.Fatalf("HTTPMaxBodyBytes = %d, want 2048 (from an unquoted TOML integer)", cfg.HTTPMaxBodyBytes)
	}
}
