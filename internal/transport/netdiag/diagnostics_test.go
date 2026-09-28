package netdiag

import (
	"context"
	"net"
	"testing"
)

func TestCheckPortAvailability_Available(t *testing.T) {
	item := CheckPortAvailability("tcp", "127.0.0.1:0")
	if item.Status != StatusOK {
		t.Errorf("status = %v, want ok; message = %s", item.Status, item.Message)
	}
}

func TestCheckPortAvailability_InUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer func() { _ = l.Close() }()

	item := CheckPortAvailability("tcp", l.Addr().String())
	if item.Status != StatusError {
		t.Errorf("status = %v, want error; message = %s", item.Status, item.Message)
	}
	if item.Category != "port" {
		t.Errorf("category = %v, want port", item.Category)
	}
	if item.Action == "" {
		t.Error("expected non-empty actionable resolution advice")
	}
}

func TestCheckLANInterfaces(t *testing.T) {
	ips, item := CheckLANInterfaces()
	if item.Category != "interface" {
		t.Errorf("category = %s, want interface", item.Category)
	}
	t.Logf("Found %d LAN IPs: %v, status: %s, message: %s", len(ips), ips, item.Status, item.Message)
}

func TestRunDiagnostics_LoopbackOnly(t *testing.T) {
	ctx := context.Background()
	diag := RunDiagnostics(ctx, DiagnosticsConfig{
		BindAddress: "127.0.0.1:8080",
		Hostname:    "alexandryn.local",
		Scheme:      "http",
	})

	if diag.Reachability != "loopback" {
		t.Errorf("reachability = %s, want loopback", diag.Reachability)
	}
	if diag.LANVerified {
		t.Error("expected LANVerified to be false when bound to loopback")
	}
	if diag.HostnameVerified {
		t.Error("expected HostnameVerified to be false when bound to loopback")
	}
	if len(diag.ActiveAddresses) == 0 || diag.ActiveAddresses[0] != "http://127.0.0.1:8080" {
		t.Errorf("activeAddresses = %v, want http://127.0.0.1:8080", diag.ActiveAddresses)
	}
}

func TestRunDiagnostics_LANBind(t *testing.T) {
	ctx := context.Background()
	diag := RunDiagnostics(ctx, DiagnosticsConfig{
		BindAddress: "192.168.1.100:8080",
		Hostname:    "alexandryn.local",
		Scheme:      "http",
	})

	if diag.Reachability != "lan" {
		t.Errorf("reachability = %s, want lan", diag.Reachability)
	}
	// Without active mDNS server, HostnameVerified must be false
	if diag.HostnameVerified {
		t.Error("expected HostnameVerified to be false without active mDNS server")
	}
}
