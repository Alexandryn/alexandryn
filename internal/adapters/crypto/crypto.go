// Package crypto provides credential-at-rest encryption for Alexandryn:
// AES-256-GCM authenticated encryption for stored credentials, keyed by
// a single 32-byte key held in a 0600 key file under the app data directory.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// KeyLen is the AES-256 key length in bytes.
const KeyLen = 32

// Service encrypts and decrypts small secrets with a fixed key. It is
// safe for concurrent use — the AEAD it holds is stateless and every
// call generates its own nonce.
type Service struct {
	aead cipher.AEAD
	key  []byte
}

// NewService builds a Service from a 32-byte key. A key of any other
// length is rejected rather than stretched or truncated — the caller
// (keyfile.LoadOrCreateKey) is responsible for producing a real
// AES-256 key.
func NewService(key []byte) (*Service, error) {
	if len(key) != KeyLen {
		return nil, fmt.Errorf("crypto: key must be %d bytes, got %d", KeyLen, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	dup := make([]byte, KeyLen)
	copy(dup, key)
	return &Service{aead: aead, key: dup}, nil
}

// ErrDecrypt is returned by Decrypt when the ciphertext fails
// authentication (e.g. corrupted data, truncated value, or mismatched key).
var ErrDecrypt = errors.New("crypto: credential could not be decrypted")

// Encrypt returns the GCM ciphertext and the fresh random nonce used to
// produce it. The two are stored in separate columns and passed back to
// Decrypt together.
func (s *Service) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	ciphertext = s.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt reverses Encrypt. It returns ErrDecrypt for any authentication
// failure and never a partially-decrypted result.
func (s *Service) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != s.aead.NonceSize() {
		return nil, ErrDecrypt
	}
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

// DeriveSubkey returns a 32-byte key derived from the service's master key
// via HKDF-SHA256 with the given info context string, allowing distinct
// subkeys for purposes like cursor HMAC signing without additional key files.
func (s *Service) DeriveSubkey(info string) ([]byte, error) {
	out, err := hkdf.Key(sha256.New, s.key, nil, info, KeyLen)
	if err != nil {
		return nil, fmt.Errorf("crypto: derive subkey: %w", err)
	}
	return out, nil
}
