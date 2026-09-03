package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/config"
)

// TestRun_ServesTLSWhenConfigHasACert is phase 13 Tier 0: when config
// resolves an in-process-TLS bind (here, a private bind with the opt-in
// TLS_CERT_FILE/TLS_KEY_FILE), cmd/server wraps the listener in TLS — a
// plaintext request to the port fails, an https request succeeds. This is
// what lets validateBindAddress accept a public bind without the process
// silently serving plaintext on it.
func TestRun_ServesTLSWhenConfigHasACert(t *testing.T) {
	certPEM, keyPEM := selfSignedCert(t)
	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/tls/cert.pem":
			return certPEM, nil
		case "/tls/key.pem":
			return keyPEM, nil
		default:
			return nil, fs.ErrNotExist
		}
	}

	t.Setenv("OPEN_LIBRARY_USER_AGENT", "Alexandryn/test")
	t.Setenv("BIND_ADDRESS", "127.0.0.1:0")
	t.Setenv("TLS_CERT_FILE", "/tls/cert.pem")
	t.Setenv("TLS_KEY_FILE", "/tls/key.pem")

	cfg, err := config.Load("", readFile, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.TLSCertificate() == nil {
		t.Fatal("config.Load did not populate a TLS certificate for the opt-in private bind")
	}

	addrCh := make(chan string, 1)
	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           quietLogger,
		newRouter:           newProductionRouter,
		listen:              realListenDeps(addrCh),
		newServer:           realServerDeps(),
		clock:               realClock{},
		obtainPostgres:      func(context.Context, *config.Config) error { return nil },
		postgresMaxAttempts: 1,
		postgresBackoff:     0,
		sleep:               sleepOrDone,
		runMigrations:       func(context.Context, *config.Config) error { return nil },
		newPool:             func(context.Context, *config.Config) (pgPool, *repositories, error) { return &fakePool{}, nil, nil },
		stderr:              io.Discard,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exitCh := make(chan int, 1)
	go func() { exitCh <- run(ctx, deps) }()

	addr := waitForAddr(t, addrCh, 2*time.Second)

	// Plaintext to a TLS listener never returns a real 200 — Go's TLS
	// server answers a plaintext request with 400 Bad Request (or the
	// client errors on the handshake).
	if resp, err := http.Get("http://" + addr + "/healthz"); err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("plaintext GET returned 200 — the listener is not TLS")
		}
	}

	// https with a client that trusts the self-signed test cert succeeds.
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}},
	}
	var ok bool
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get("https://" + addr + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ok = true
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ok {
		t.Fatal("https GET /healthz never returned 200 over the in-process TLS listener")
	}

	cancel()
	if code := waitForExit(t, exitCh, 3*time.Second); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func selfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "alexandryn-test"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
}
