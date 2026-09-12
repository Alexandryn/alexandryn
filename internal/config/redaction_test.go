package config_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
)

const secretDSN = "postgres://user:s3cr3t@host/db"

// --- RedactedString's own mechanics ---

func TestRedactedString_StringIsRedacted(t *testing.T) {
	if got := config.RedactedString(secretDSN).String(); got != "[redacted]" {
		t.Fatalf("String() = %q, want the redacted placeholder", got)
	}
}

func TestRedactedString_LogValueIsRedacted(t *testing.T) {
	got := config.RedactedString(secretDSN).LogValue().String()
	if got != "[redacted]" {
		t.Fatalf("LogValue().String() = %q, want the redacted placeholder", got)
	}
}

func TestRedactedString_MarshalJSONIsRedacted(t *testing.T) {
	data, err := json.Marshal(config.RedactedString(secretDSN))
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if got := string(data); got != `"[redacted]"` {
		t.Fatalf("MarshalJSON output = %s, want the redacted placeholder", got)
	}
}

func TestRedactedString_RevealReturnsTheRealValue(t *testing.T) {
	if got := config.RedactedString(secretDSN).Reveal(); got != secretDSN {
		t.Fatalf("Reveal() = %q, want the real underlying value", got)
	}
}

// Tests verifying that structured logging with slog.JSONHandler preserves redaction.

func TestConfigRedaction_FieldLoggedDirectly(t *testing.T) {
	cfg := &config.Config{DatabaseURL: config.RedactedString(secretDSN)}

	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config loaded", "database_url", cfg.DatabaseURL)

	out := buf.String()
	if strings.Contains(out, secretDSN) {
		t.Fatalf("real DATABASE_URL value leaked into log output: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("log output doesn't contain the redacted placeholder: %s", out)
	}
}

// logValuerOnlyString implements only slog.LogValuer without json.Marshaler,
// used to demonstrate why implementing both interfaces is required.
type logValuerOnlyString string

func (s logValuerOnlyString) LogValue() slog.Value { return slog.StringValue("[redacted]") }

func TestConfigRedaction_WholeStructAsOneAttribute(t *testing.T) {
	// Verify that a LogValuer-only type would leak when the containing struct
	// is marshaled via JSON reflection:
	type logValuerOnlyStub struct {
		DatabaseURL logValuerOnlyString
	}
	stub := logValuerOnlyStub{DatabaseURL: logValuerOnlyString(secretDSN)}

	var stubBuf bytes.Buffer
	slog.New(slog.NewJSONHandler(&stubBuf, nil)).Info("config loaded", "config", stub)
	if !strings.Contains(stubBuf.String(), secretDSN) {
		t.Fatalf("test assumption broken: expected the LogValuer-only stub to leak through the JSON reflection fallback, so this test can't prove it detects the real gap.\noutput: %s", stubBuf.String())
	}

	// Now verify that Config, which implements both interfaces, does not leak.
	cfg := &config.Config{DatabaseURL: config.RedactedString(secretDSN)}

	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config loaded", "config", cfg)
	out := buf.String()
	if strings.Contains(out, secretDSN) {
		t.Fatalf("Config leaked DatabaseURL's real value when logged as a whole struct via slog.Any: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("log output doesn't contain the redacted placeholder anywhere: %s", out)
	}
}

// Redaction on load failure:

func TestLoad_FailedLoadNeverIncludesDatabaseURLInTheError(t *testing.T) {
	t.Setenv("OPEN_LIBRARY_USER_AGENT", "Alexandryn/dev (test)")
	t.Setenv("DATABASE_URL", secretDSN)
	t.Setenv("LOG_LEVEL", "verbose") // unrelated, invalid

	_, err := config.Load("", noFile, fakeUserConfigDir)
	if err == nil {
		t.Fatal("Load() error = nil, want the LOG_LEVEL validation error")
	}
	if strings.Contains(err.Error(), secretDSN) {
		t.Fatalf("error string leaked DATABASE_URL's real value: %q", err.Error())
	}
}
