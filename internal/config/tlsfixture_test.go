package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// generateCert builds a self-signed certificate/key PEM pair valid across
// [notBefore, notAfter), for tests only. A fixed past or future window
// (rather than relative to time.Now()) makes an expired/not-yet-valid
// fixture deterministic without needing to inject a clock into FR-8's
// validation — the fixture's own dates already sit on the correct side of
// "now" for any runtime this test could plausibly execute on.
func generateCert(t *testing.T, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "alexandryn-test"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM
}

// validCert returns a cert/key pair valid for the next 24h.
func validCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	now := time.Now()
	return generateCert(t, now.Add(-time.Hour), now.Add(24*time.Hour))
}

// expiredCert returns a cert/key pair whose validity window is fixed
// safely in the past.
func expiredCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	return generateCert(t, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2000, 6, 1, 0, 0, 0, 0, time.UTC))
}
