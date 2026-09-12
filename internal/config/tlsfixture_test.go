package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"
)

// generateCert builds a self-signed certificate/key PEM pair valid across
// [notBefore, notAfter), for tests only.
func generateCert(t *testing.T, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	return generateCertSAN(t, notBefore, notAfter, nil)
}

// generateCertSAN builds a test certificate/key pair with explicit Subject Alternative Names.
func generateCertSAN(t *testing.T, notBefore, notAfter time.Time, sans []string) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	var dnsNames []string
	var ipAddrs []net.IP
	for _, s := range sans {
		if ip := net.ParseIP(s); ip != nil {
			ipAddrs = append(ipAddrs, ip)
		} else {
			dnsNames = append(dnsNames, s)
		}
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "alexandryn-test"},
		DNSNames:              dnsNames,
		IPAddresses:           ipAddrs,
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

// certWithSAN returns a cert/key valid for the next 24h with the given
// subjectAltName (DNS name or IP string).
func certWithSAN(t *testing.T, san string) (certPEM, keyPEM []byte) {
	t.Helper()
	now := time.Now()
	return generateCertSAN(t, now.Add(-time.Hour), now.Add(24*time.Hour), []string{san})
}
