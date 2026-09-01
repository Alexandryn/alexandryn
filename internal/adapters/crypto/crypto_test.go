package crypto_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
)

func newKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, crypto.KeyLen)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

func TestService_EncryptDecryptRoundTrip(t *testing.T) {
	svc, err := crypto.NewService(newKey(t))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	plaintext := []byte(`{"username":"reader","password":"s3cr3t p@ss"}`)
	ct, nonce, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(ct, []byte("reader")) || bytes.Contains(ct, []byte("s3cr3t")) {
		t.Fatal("ciphertext contains plaintext fragments")
	}

	got, err := svc.Decrypt(ct, nonce)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round trip = %q, want %q", got, plaintext)
	}
}

func TestService_EncryptUsesFreshNonce(t *testing.T) {
	svc, _ := crypto.NewService(newKey(t))
	_, n1, _ := svc.Encrypt([]byte("x"))
	_, n2, _ := svc.Encrypt([]byte("x"))
	if bytes.Equal(n1, n2) {
		t.Fatal("two Encrypt calls reused the same nonce")
	}
}

func TestService_DecryptRejectsTamperedCiphertext(t *testing.T) {
	svc, _ := crypto.NewService(newKey(t))
	ct, nonce, _ := svc.Encrypt([]byte("hello world"))

	ct[len(ct)-1] ^= 0xff
	if _, err := svc.Decrypt(ct, nonce); !errors.Is(err, crypto.ErrDecrypt) {
		t.Fatalf("tampered ciphertext: err = %v, want ErrDecrypt", err)
	}
}

func TestService_DecryptRejectsWrongKey(t *testing.T) {
	a, _ := crypto.NewService(newKey(t))
	b, _ := crypto.NewService(newKey(t))

	ct, nonce, _ := a.Encrypt([]byte("hello world"))
	if _, err := b.Decrypt(ct, nonce); !errors.Is(err, crypto.ErrDecrypt) {
		t.Fatalf("wrong key: err = %v, want ErrDecrypt", err)
	}
}

func TestService_DecryptRejectsBadNonce(t *testing.T) {
	svc, _ := crypto.NewService(newKey(t))
	ct, _, _ := svc.Encrypt([]byte("hello world"))
	if _, err := svc.Decrypt(ct, []byte("short")); !errors.Is(err, crypto.ErrDecrypt) {
		t.Fatalf("bad nonce: err = %v, want ErrDecrypt", err)
	}
}

func TestNewService_RejectsWrongKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, err := crypto.NewService(make([]byte, n)); err == nil {
			t.Errorf("NewService(%d-byte key): err = nil, want rejection", n)
		}
	}
}

func TestService_DeriveSubkeyIsDeterministicAndDistinct(t *testing.T) {
	key := newKey(t)
	a, _ := crypto.NewService(key)
	b, _ := crypto.NewService(key)

	k1, err := a.DeriveSubkey("cursor-hmac-v1")
	if err != nil {
		t.Fatalf("DeriveSubkey: %v", err)
	}
	k2, _ := b.DeriveSubkey("cursor-hmac-v1")
	if !bytes.Equal(k1, k2) {
		t.Fatal("same key + same info produced different subkeys")
	}
	if len(k1) != crypto.KeyLen {
		t.Fatalf("subkey len = %d, want %d", len(k1), crypto.KeyLen)
	}

	kOther, _ := a.DeriveSubkey("something-else")
	if bytes.Equal(k1, kOther) {
		t.Fatal("different info labels produced the same subkey")
	}
	if bytes.Equal(k1, key) {
		t.Fatal("subkey equals the root key")
	}
}
