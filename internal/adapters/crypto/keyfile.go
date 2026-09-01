package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// KeyFileName is the credential key's filename within the app-data
// directory (backend-source-adapter.md FR-13).
const KeyFileName = "source-credentials.key"

// CredentialCounter reports how many stored sources currently hold an
// encrypted credential. LoadOrCreateKey uses it to tell a genuine first
// run (generate a key silently) from a lost key file with credentialed
// sources still present (generate a replacement, but log a warning
// first) — FR-13's exact distinction.
type CredentialCounter func() (int, error)

// KeyPath returns the credential key file's path under appDataDir.
func KeyPath(appDataDir string) string {
	return filepath.Join(appDataDir, KeyFileName)
}

// LoadOrCreateKey reads the credential key from appDataDir, or creates
// one following FR-13's rules:
//
//   - key file present and 32 bytes: return it.
//   - key file absent, no credentialed sources: genuine first run (or
//     every credentialed source was removed) — generate a key silently,
//     write it 0600.
//   - key file absent, at least one credentialed source: the key was
//     lost. Log a warn line with the affected count (never labels),
//     then generate a replacement 0600 key. Every existing ciphertext
//     is now permanently undecryptable, which surfaces per-source as
//     that source's next health check failing with "auth-rejected".
//
// countCredentialed may be nil, in which case a missing key file is
// always treated as first run (the only caller that passes nil is a
// context with no persistence yet).
func LoadOrCreateKey(appDataDir string, countCredentialed CredentialCounter, logger *slog.Logger) ([]byte, error) {
	path := KeyPath(appDataDir)

	key, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(key) != KeyLen {
			return nil, fmt.Errorf("crypto: key file %s is %d bytes, expected %d — refusing to use a malformed key", path, len(key), KeyLen)
		}
		return key, nil
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("crypto: reading key file: %w", err)
	}

	// Key file absent.
	if countCredentialed != nil {
		n, countErr := countCredentialed()
		if countErr != nil {
			return nil, fmt.Errorf("crypto: checking for credentialed sources: %w", countErr)
		}
		if n > 0 && logger != nil {
			logger.Warn(
				"source credential key file is missing but credentialed sources exist — generating a new key; affected sources will need their credential re-entered",
				slog.Int("affectedSources", n),
			)
		}
	}

	newKey := make([]byte, KeyLen)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("crypto: generating key: %w", err)
	}
	if err := writeKeyFile(path, newKey); err != nil {
		return nil, err
	}
	return newKey, nil
}

// writeKeyFile writes key to path with 0600 permissions set at creation
// time (O_EXCL so a concurrent creator loses rather than silently
// sharing), creating the parent directory if needed.
func writeKeyFile(path string, key []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("crypto: creating key directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("crypto: creating key file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(key); err != nil {
		return fmt.Errorf("crypto: writing key file: %w", err)
	}
	return nil
}
