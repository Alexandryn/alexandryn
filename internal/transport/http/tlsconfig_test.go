package http_test

import (
	"crypto/tls"
	"testing"

	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func TestNewTLSConfig_Policy(t *testing.T) {
	c := transporthttp.NewTLSConfig()

	if c.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want TLS 1.2", c.MinVersion)
	}

	want := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	}
	if len(c.CipherSuites) != len(want) {
		t.Fatalf("CipherSuites = %v, want %v", c.CipherSuites, want)
	}
	for i := range want {
		if c.CipherSuites[i] != want[i] {
			t.Errorf("CipherSuites[%d] = %x, want %x", i, c.CipherSuites[i], want[i])
		}
	}
	// All must be AEAD (no CBC) — cross-check against Go's insecure list.
	for _, id := range c.CipherSuites {
		for _, ins := range tls.InsecureCipherSuites() {
			if id == ins.ID {
				t.Errorf("cipher %x is on Go's insecure list", id)
			}
		}
	}

	if len(c.NextProtos) != 2 || c.NextProtos[0] != "h2" || c.NextProtos[1] != "http/1.1" {
		t.Errorf("NextProtos = %v, want [h2 http/1.1]", c.NextProtos)
	}
}
