package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/logging"
)

func TestNew_ProducesJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("info", &buf)

	logger.Info("server ready")

	out := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("output isn't JSON: %s", out)
	}
	if !strings.Contains(out, "server ready") {
		t.Fatalf("output missing the message: %s", out)
	}
}

func TestNew_DefaultLevelInfoSuppressesDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("info", &buf)

	logger.Debug("verbose per-step detail")
	logger.Info("server ready")

	out := buf.String()
	if strings.Contains(out, "verbose per-step detail") {
		t.Fatalf("debug line emitted at the default info level: %s", out)
	}
	if !strings.Contains(out, "server ready") {
		t.Fatalf("info line missing: %s", out)
	}
}

func TestNew_DebugLevelEmitsDebugLines(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("debug", &buf)

	logger.Debug("verbose per-step detail")

	if !strings.Contains(buf.String(), "verbose per-step detail") {
		t.Fatalf("debug line missing when level is debug: %s", buf.String())
	}
}

func TestNew_WarnLevelSuppressesInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("warn", &buf)

	logger.Info("server ready")
	logger.Warn("retried connection attempt")

	out := buf.String()
	if strings.Contains(out, "server ready") {
		t.Fatalf("info line emitted at warn level: %s", out)
	}
	if !strings.Contains(out, "retried connection attempt") {
		t.Fatalf("warn line missing: %s", out)
	}
}

func TestNew_ErrorLevelSuppressesWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("error", &buf)

	logger.Warn("retried connection attempt")
	logger.Error("request failed")

	out := buf.String()
	if strings.Contains(out, "retried connection attempt") {
		t.Fatalf("warn line emitted at error level: %s", out)
	}
	if !strings.Contains(out, "request failed") {
		t.Fatalf("error line missing: %s", out)
	}
}

func TestNew_UnrecognizedLevelStringDefaultsToInfo(t *testing.T) {
	// config.Load already validates LOG_LEVEL to one of the four enum
	// values before a Config exists at all — this proves New() degrades
	// safely rather than panicking if ever called with something else.
	var buf bytes.Buffer
	logger := logging.New("", &buf)

	logger.Debug("should not appear")
	logger.Info("should appear")

	out := buf.String()
	if strings.Contains(out, "should not appear") {
		t.Fatalf("debug line emitted for an unrecognized level string: %s", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Fatalf("info line missing: %s", out)
	}
}

// Tests verifying that RedactedString fields are redacted when logged through the structured logger:

const secretDSN = "postgres://user:s3cr3t@host/db"

func TestNew_RedactedValueLoggedDirectlyNeverLeaks(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("info", &buf)

	logger.Info("config loaded", "database_url", config.RedactedString(secretDSN))

	out := buf.String()
	if strings.Contains(out, secretDSN) {
		t.Fatalf("real value leaked through the constructed logger: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("output doesn't contain the redacted placeholder: %s", out)
	}
}

func TestNew_RedactedValueLoggedAsPartOfAContainingStructNeverLeaks(t *testing.T) {
	type wrapper struct {
		DatabaseURL config.RedactedString
	}

	var buf bytes.Buffer
	logger := logging.New("info", &buf)

	logger.Info("config loaded", "config", wrapper{DatabaseURL: config.RedactedString(secretDSN)})

	out := buf.String()
	if strings.Contains(out, secretDSN) {
		t.Fatalf("real value leaked when logged as part of a containing struct: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("output doesn't contain the redacted placeholder: %s", out)
	}
}
