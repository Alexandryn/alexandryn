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
// resolves an in-process-TLS bind, cmd/server wraps the listener in TLS —
// a plaintext request to the port fails, an https request succeeds. Two
// cases: a private bind with the opt-in TLS_CERT_FILE/TLS_KEY_FILE, and
// an accepted public bind (0.0.0.0, classPublic). The public case is the
// safety property validateBindAddress's loosened check depends on — a
// public bind that Load accepts must actually be served over TLS by
// run(), never plaintext.
func TestRun_ServesTLSWhenConfigHasACert(t *testing.T) {
	t.Run("private opt-in cert", func(t *testing.T) {
		assertRunServesTLS(t, "127.0.0.1:0")
	})
	t.Run("accepted public bind", func(t *testing.T) {
		assertRunServesTLS(t, "0.0.0.0:0")
	})
}

func assertRunServesTLS(t *testing.T, bindAddr string) {
	t.Helper()
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
	t.Setenv("BIND_ADDRESS", bindAddr)
	t.Setenv("TLS_CERT_FILE", "/tls/cert.pem")
	t.Setenv("TLS_KEY_FILE", "/tls/key.pem")

	cfg, err := config.Load("", readFile, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.TLSCertificate() == nil {
		t.Fatalf("config.Load did not populate a TLS certificate for bind %q", bindAddr)
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
	// An unspecified bind (0.0.0.0 / ::) reports its listen address as
	// 0.0.0.0:port; dial it on the loopback interface.
	if h, p, err := net.SplitHostPort(addr); err == nil {
		if ip := net.ParseIP(h); ip != nil && ip.IsUnspecified() {
			addr = net.JoinHostPort("127.0.0.1", p)
		}
	}

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

// TestRun_PublicStaticBind_Serves80Redirect: a public static-cert bind
// (Mode A) also runs a :80 listener that 308-redirects http:// to
// https:// (backend-network-transport.md FR-3). The listen dep here maps
// ":80" to an ephemeral loopback port so the test can exercise it
// unprivileged.
func TestRun_PublicStaticBind_Serves80Redirect(t *testing.T) {
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
	t.Setenv("BIND_ADDRESS", "0.0.0.0:0")
	t.Setenv("TLS_CERT_FILE", "/tls/cert.pem")
	t.Setenv("TLS_KEY_FILE", "/tls/key.pem")

	cfg, err := config.Load("", readFile, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	mainCh := make(chan string, 1)
	redirectCh := make(chan string, 1)
	listen := func(network, address string) (net.Listener, error) {
		if address == ":80" {
			ln, lerr := net.Listen("tcp", "127.0.0.1:0")
			if lerr == nil {
				redirectCh <- ln.Addr().String()
			}
			return ln, lerr
		}
		ln, lerr := net.Listen(network, address)
		if lerr == nil {
			mainCh <- ln.Addr().String()
		}
		return ln, lerr
	}

	deps := runDeps{
		loadConfig:          func() (*config.Config, error) { return cfg, nil },
		newLogger:           quietLogger,
		newRouter:           newProductionRouter,
		listen:              listen,
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

	_ = waitForAddr(t, mainCh, 2*time.Second)
	redirectAddr := waitForAddr(t, redirectCh, 2*time.Second)

	client := &http.Client{
		Timeout:       2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	var resp *http.Response
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var gerr error
		resp, gerr = client.Get("http://" + redirectAddr + "/library")
		if gerr == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if resp == nil {
		t.Fatal(":80 redirect listener never answered")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want 308", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc == "" || loc[:6] != "https:" {
		t.Fatalf("Location = %q, want an https:// URL", loc)
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
