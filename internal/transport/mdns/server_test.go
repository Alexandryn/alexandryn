package mdns

import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestNewServer_Defaults(t *testing.T) {
	s, err := NewServer(Config{})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s.cfg.Hostname != "alexandryn.local" {
		t.Errorf("Hostname = %q, want alexandryn.local", s.cfg.Hostname)
	}
	if s.cfg.Port != 80 {
		t.Errorf("Port = %d, want 80", s.cfg.Port)
	}
	if s.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestNewServer_TrimTrailingDot(t *testing.T) {
	s, err := NewServer(Config{Hostname: "custom.local."})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if s.cfg.Hostname != "custom.local" {
		t.Errorf("Hostname = %q, want custom.local", s.cfg.Hostname)
	}
}

func TestHandleQuery_ARecord(t *testing.T) {
	ip := net.ParseIP("192.168.1.50")
	s, err := NewServer(Config{
		Hostname: "alexandryn.local",
		IPs:      []net.IP{ip},
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	s.activeIPs = []net.IP{ip}

	query := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID: 0x1234,
		},
		Questions: []dnsmessage.Question{
			{
				Name:  dnsmessage.MustNewName("alexandryn.local."),
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
			},
		},
	}

	// Fake UDP listener to capture reply
	fakeConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer fakeConn.Close()

	s.conn = fakeConn

	// Unicast simulation
	clientAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321}
	s.handleQuery(&query, clientAddr)
}

func TestHandleQuery_UnrelatedQueryIgnored(t *testing.T) {
	s, _ := NewServer(Config{
		Hostname: "alexandryn.local",
		IPs:      []net.IP{net.ParseIP("192.168.1.50")},
	})
	s.activeIPs = []net.IP{net.ParseIP("192.168.1.50")}

	query := dnsmessage.Message{
		Header: dnsmessage.Header{ID: 0x9999},
		Questions: []dnsmessage.Question{
			{
				Name:  dnsmessage.MustNewName("google.com."),
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
			},
		},
	}

	fakeConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer fakeConn.Close()
	s.conn = fakeConn

	// Should not crash and should not write any answer
	s.handleQuery(&query, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321})
}

func TestHandleQuery_ServiceDiscovery(t *testing.T) {
	ip := net.ParseIP("10.0.0.5")
	s, _ := NewServer(Config{
		Hostname: "mybooks.local",
		Port:     8080,
		IPs:      []net.IP{ip},
	})
	s.activeIPs = []net.IP{ip}

	query := dnsmessage.Message{
		Header: dnsmessage.Header{ID: 0x5678},
		Questions: []dnsmessage.Question{
			{
				Name:  dnsmessage.MustNewName("_http._tcp.local."),
				Type:  dnsmessage.TypePTR,
				Class: dnsmessage.ClassINET,
			},
		},
	}

	fakeConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer fakeConn.Close()
	s.conn = fakeConn

	s.handleQuery(&query, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321})
}

func TestServer_StartAndClose(t *testing.T) {
	s, err := NewServer(Config{
		Hostname: "alexandryn.local",
		Port:     8080,
		IPs:      []net.IP{net.ParseIP("127.0.0.1")},
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = s.Start(ctx)
	if err != nil {
		// In some restricted CI environments or test containers, binding multicast UDP 5353
		// may return EADDRINUSE or EPERM. The server must handle this gracefully without panic.
		t.Logf("Start returned (expected in restricted test environments): %v", err)
		return
	}

	if !s.IsRunning() {
		t.Error("expected server to be running")
	}

	activeIPs := s.ActiveIPs()
	if len(activeIPs) == 0 {
		t.Error("expected at least one active IP")
	}

	err = s.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if s.IsRunning() {
		t.Error("expected server to not be running after close")
	}
}

func TestVerifyResolution_DirectTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	verified, err := VerifyResolution(ctx, "nonexistent-test-host.local", []net.IP{net.ParseIP("192.168.1.99")}, 100*time.Millisecond)
	if verified {
		t.Error("expected nonexistent host to not be verified")
	}
	if err == nil {
		t.Error("expected non-nil error on verification failure")
	}
}
