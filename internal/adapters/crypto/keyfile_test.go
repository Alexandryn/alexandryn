package crypto_test

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
)

func TestLoadOrCreateKey_FirstRunGeneratesSilently(t *testing.T) {
	dir := t.TempDir()
	var logbuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logbuf, nil))

	key, err := crypto.LoadOrCreateKey(dir, func() (int, error) { return 0, nil }, logger)
	if err != nil {
		t.Fatalf("LoadOrCreateKey: %v", err)
	}
	if len(key) != crypto.KeyLen {
		t.Fatalf("key len = %d, want %d", len(key), crypto.KeyLen)
	}
	if logbuf.Len() != 0 {
		t.Fatalf("first run logged something: %q", logbuf.String())
	}

	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(crypto.KeyPath(dir))
		if statErr != nil {
			t.Fatalf("stat key file: %v", statErr)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("key file mode = %o, want 600", perm)
		}
	}
}

func TestLoadOrCreateKey_ReturnsExistingKey(t *testing.T) {
	dir := t.TempDir()
	first, err := crypto.LoadOrCreateKey(dir, func() (int, error) { return 0, nil }, nil)
	if err != nil {
		t.Fatalf("first LoadOrCreateKey: %v", err)
	}
	second, err := crypto.LoadOrCreateKey(dir, func() (int, error) { return 3, nil }, nil)
	if err != nil {
		t.Fatalf("second LoadOrCreateKey: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("second call generated a new key instead of returning the existing one")
	}
}

func TestLoadOrCreateKey_MissingKeyWithCredentialedSourcesWarns(t *testing.T) {
	dir := t.TempDir()
	var logbuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logbuf, nil))

	key, err := crypto.LoadOrCreateKey(dir, func() (int, error) { return 4, nil }, logger)
	if err != nil {
		t.Fatalf("LoadOrCreateKey: %v", err)
	}
	if len(key) != crypto.KeyLen {
		t.Fatalf("no replacement key generated")
	}

	out := logbuf.String()
	if !bytes.Contains(logbuf.Bytes(), []byte("level=WARN")) {
		t.Fatalf("expected a WARN line, got: %q", out)
	}
	if !bytes.Contains(logbuf.Bytes(), []byte("affectedSources=4")) {
		t.Fatalf("warn line missing the affected count: %q", out)
	}
	// FR-13: never a source label in the line.
	if bytes.Contains(logbuf.Bytes(), []byte("label")) {
		t.Fatalf("warn line leaked a source label: %q", out)
	}
}

func TestLoadOrCreateKey_RejectsMalformedKeyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(crypto.KeyPath(dir), []byte("too short"), 0o600); err != nil {
		t.Fatalf("seed bad key: %v", err)
	}
	if _, err := crypto.LoadOrCreateKey(dir, func() (int, error) { return 0, nil }, nil); err == nil {
		t.Fatal("LoadOrCreateKey accepted a malformed key file")
	}
}

func TestLoadOrCreateKey_PropagatesCounterError(t *testing.T) {
	dir := t.TempDir()
	sentinel := errors.New("db down")
	if _, err := crypto.LoadOrCreateKey(dir, func() (int, error) { return 0, sentinel }, nil); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want it to wrap the counter error", err)
	}
	if _, statErr := os.Stat(crypto.KeyPath(dir)); statErr == nil {
		t.Fatal("a key file was written despite the counter failing")
	}
}

func TestLoadOrCreateKey_KeyRoundTripsThroughService(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "appdata")
	key, err := crypto.LoadOrCreateKey(dir, nil, nil)
	if err != nil {
		t.Fatalf("LoadOrCreateKey: %v", err)
	}
	svc, err := crypto.NewService(key)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ct, nonce, _ := svc.Encrypt([]byte("secret"))
	got, err := svc.Decrypt(ct, nonce)
	if err != nil || string(got) != "secret" {
		t.Fatalf("round trip failed: %q / %v", got, err)
	}
}
