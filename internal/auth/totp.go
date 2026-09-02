package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type TOTPEngine struct {
	issuer   string
	timeStep time.Duration
	digits   int
}

func NewTOTPEngine(issuer string) *TOTPEngine {
	return &TOTPEngine{
		issuer:   issuer,
		timeStep: 30 * time.Second,
		digits:   6,
	}
}

// GenerateSecret produces a cryptographically random 20-byte base32-encoded secret.
func (e *TOTPEngine) GenerateSecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("auth/totp: generate secret failed: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// GenerateCode computes the 6-digit TOTP code for the given secret at time t (RFC 6238).
func (e *TOTPEngine) GenerateCode(secretBase32 string, t time.Time) (string, error) {
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secretBase32)))
	if err != nil {
		// Fallback for padded base32
		secretBytes, err = base32.StdEncoding.DecodeString(strings.ToUpper(strings.TrimSpace(secretBase32)))
		if err != nil {
			return "", fmt.Errorf("auth/totp: decode base32 secret failed: %w", err)
		}
	}

	counter := uint64(t.Unix()) / uint64(e.timeStep.Seconds())
	return e.generateHOTP(secretBytes, counter)
}

// ValidateCode checks if code matches the secret at time t within ±1 time step (RFC 6238).
func (e *TOTPEngine) ValidateCode(secretBase32, code string, t time.Time) (bool, error) {
	code = strings.TrimSpace(code)
	if len(code) != e.digits {
		return false, nil
	}

	for _, offset := range []int{0, -1, 1} {
		testTime := t.Add(time.Duration(offset) * e.timeStep)
		expectedCode, err := e.GenerateCode(secretBase32, testTime)
		if err != nil {
			return false, err
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(expectedCode)) == 1 {
			return true, nil
		}
	}
	return false, nil
}

// KeyURI constructs an otpauth:// URI for authenticator applications.
func (e *TOTPEngine) KeyURI(accountName, secretBase32 string) string {
	v := url.Values{}
	v.Set("secret", strings.ToUpper(strings.TrimSpace(secretBase32)))
	v.Set("issuer", e.issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")

	label := e.issuer + ":" + accountName
	return fmt.Sprintf("otpauth://totp/%s?%s", label, v.Encode())
}

// GenerateRecoveryCodes produces count random recovery codes and their SHA-256 hex hashes.
func (e *TOTPEngine) GenerateRecoveryCodes(count int) (codes []string, hashes []string, err error) {
	const charset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	codes = make([]string, count)
	hashes = make([]string, count)

	for i := 0; i < count; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, fmt.Errorf("auth/totp: generate recovery code failed: %w", err)
		}
		var codeBuilder strings.Builder
		for j := 0; j < 8; j++ {
			codeBuilder.WriteByte(charset[int(b[j])%len(charset)])
			if j == 3 {
				codeBuilder.WriteByte('-')
			}
		}
		code := codeBuilder.String()
		codes[i] = code
		h := sha256.Sum256([]byte(code))
		hashes[i] = hex.EncodeToString(h[:])
	}
	return codes, hashes, nil
}

// VerifyRecoveryCode checks if rawCode matches any stored hash and returns the matched index (-1 if none).
func (e *TOTPEngine) VerifyRecoveryCode(rawCode string, storedHashes []string) int {
	rawCode = strings.TrimSpace(strings.ToUpper(rawCode))
	h := sha256.Sum256([]byte(rawCode))
	hashHex := hex.EncodeToString(h[:])

	for i, stored := range storedHashes {
		if subtle.ConstantTimeCompare([]byte(hashHex), []byte(stored)) == 1 {
			return i
		}
	}
	return -1
}

func (e *TOTPEngine) generateHOTP(secret []byte, counter uint64) (string, error) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf)
	h := mac.Sum(nil)

	offset := h[len(h)-1] & 0x0f
	binCode := (int(h[offset]&0x7f) << 24) |
		(int(h[offset+1]&0xff) << 16) |
		(int(h[offset+2]&0xff) << 8) |
		(int(h[offset+3] & 0xff))

	otp := binCode % 1000000
	return fmt.Sprintf("%06d", otp), nil
}

// EncryptSecret encrypts plaintext using AES-256-GCM with a random 12-byte nonce prepended.
func EncryptSecret(plaintext []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		// If key is not 32 bytes, hash it to 32 bytes
		h := sha256.Sum256(key)
		key = h[:]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("auth/crypto: new cipher failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("auth/crypto: new GCM failed: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("auth/crypto: generate nonce failed: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptSecret decrypts ciphertext with AES-256-GCM.
func DecryptSecret(ciphertext []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		h := sha256.Sum256(key)
		key = h[:]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("auth/crypto: new cipher failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("auth/crypto: new GCM failed: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("auth/crypto: ciphertext too short")
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("auth/crypto: decrypt failed: %w", err)
	}

	return plaintext, nil
}
