package netdiag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/Alexandryn/alexandryn/internal/transport/mdns"
)

// CheckStatus indicates the health of a specific networking diagnostic item.
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusWarning CheckStatus = "warning"
	StatusError   CheckStatus = "error"
)

// DiagnosticItem reports a specific networking check result with actionable user advice.
type DiagnosticItem struct {
	Category string      `json:"category"` // "port", "hostname", "interface", "permission", "proxy"
	Status   CheckStatus `json:"status"`
	Message  string      `json:"message"`
	Action   string      `json:"action,omitempty"`
}

// NetworkDiagnostics summarizes the verified networking state of Alexandryn.
type NetworkDiagnostics struct {
	LANVerified      bool             `json:"lanVerified"`
	HostnameVerified bool             `json:"hostnameVerified"`
	Reachability     string           `json:"reachability"` // "loopback", "lan", "public"
	ActiveAddresses  []string         `json:"activeAddresses"`
	Items            []DiagnosticItem `json:"items"`
}

// CheckPortAvailability tests whether a TCP port can be bound.
func CheckPortAvailability(network, address string) DiagnosticItem {
	l, err := net.Listen(network, address)
	if err == nil {
		_ = l.Close()
		return DiagnosticItem{
			Category: "port",
			Status:   StatusOK,
			Message:  fmt.Sprintf("Port %s is available for binding", address),
		}
	}

	// Detect port conflict (EADDRINUSE)
	if errors.Is(err, syscall.EADDRINUSE) || strings.Contains(err.Error(), "address already in use") {
		return DiagnosticItem{
			Category: "port",
			Status:   StatusError,
			Message:  fmt.Sprintf("Port %s is already in use by another process", address),
			Action:   "Stop the conflicting process or set a different HTTP_PORT.",
		}
	}

	// Detect permission limitations (EACCES on unprivileged Linux user)
	if errors.Is(err, syscall.EACCES) || errors.Is(err, os.ErrPermission) || strings.Contains(err.Error(), "permission denied") {
		return DiagnosticItem{
			Category: "permission",
			Status:   StatusError,
			Message:  fmt.Sprintf("Permission denied binding to %s (ports < 1024 require elevated privileges)", address),
			Action:   "Run with CAP_NET_BIND_SERVICE or use an unprivileged port (e.g. 8080).",
		}
	}

	return DiagnosticItem{
		Category: "port",
		Status:   StatusWarning,
		Message:  fmt.Sprintf("Could not verify port %s: %v", address, err),
		Action:   "Verify network interface and permissions.",
	}
}

// CheckLANInterfaces discovers all active non-loopback network interfaces with private IPv4 addresses.
func CheckLANInterfaces() ([]net.IP, DiagnosticItem) {
	var lanIPs []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, DiagnosticItem{
			Category: "interface",
			Status:   StatusError,
			Message:  fmt.Sprintf("Failed to enumerate network interfaces: %v", err),
			Action:   "Verify system network configuration.",
		}
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				if ip.IsPrivate() || ip.IsLinkLocalUnicast() {
					lanIPs = append(lanIPs, ip.To4())
				}
			}
		}
	}

	if len(lanIPs) == 0 {
		return nil, DiagnosticItem{
			Category: "interface",
			Status:   StatusWarning,
			Message:  "No active local network interface found; library is accessible via loopback only",
			Action:   "Connect this device to Wi-Fi or Ethernet to enable access from other devices.",
		}
	}

	return lanIPs, DiagnosticItem{
		Category: "interface",
		Status:   StatusOK,
		Message:  fmt.Sprintf("Found %d active LAN network interface(s)", len(lanIPs)),
	}
}

// DiagnosticsConfig holds parameters for running network verification.
type DiagnosticsConfig struct {
	BindAddress  string
	Hostname     string
	Scheme       string
	MDNSServer   *mdns.Server
	VerifyRemote bool
}

// RunDiagnostics conducts a complete audit of LAN reachability and hostname verification.
func RunDiagnostics(ctx context.Context, cfg DiagnosticsConfig) NetworkDiagnostics {
	if cfg.Hostname == "" {
		cfg.Hostname = "alexandryn.local"
	}
	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}

	diag := NetworkDiagnostics{
		HostnameVerified: false,
		LANVerified:      false,
		Reachability:     "loopback",
	}

	// 1. Check Interface availability
	lanIPs, ifaceItem := CheckLANInterfaces()
	diag.Items = append(diag.Items, ifaceItem)

	// 2. Parse Bind Address to verify reachability
	host, port, err := net.SplitHostPort(cfg.BindAddress)
	if err != nil {
		diag.Items = append(diag.Items, DiagnosticItem{
			Category: "port",
			Status:   StatusError,
			Message:  fmt.Sprintf("Invalid BIND_ADDRESS %q: %v", cfg.BindAddress, err),
		})
		return diag
	}

	portSuffix := ":" + port
	if (cfg.Scheme == "http" && port == "80") || (cfg.Scheme == "https" && port == "443") {
		portSuffix = ""
	}

	isLoopback := host == "localhost" || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
	isWildcard := host == "" || host == "0.0.0.0" || host == "::" || host == "[::]"

	if isLoopback {
		diag.Reachability = "loopback"
		diag.ActiveAddresses = append(diag.ActiveAddresses, fmt.Sprintf("%s://127.0.0.1%s", cfg.Scheme, portSuffix))
		diag.Items = append(diag.Items, DiagnosticItem{
			Category: "interface",
			Status:   StatusWarning,
			Message:  "Server is bound strictly to loopback (127.0.0.1); unreachable from LAN",
			Action:   "To share across LAN, bind to 0.0.0.0 or a LAN interface.",
		})
		return diag
	}

	// If bound to LAN interface or behind a proxy/wildcard
	diag.Reachability = "lan"
	diag.LANVerified = len(lanIPs) > 0

	for _, ip := range lanIPs {
		diag.ActiveAddresses = append(diag.ActiveAddresses, fmt.Sprintf("%s://%s%s", cfg.Scheme, ip.String(), portSuffix))
	}
	if !isWildcard && host != "" {
		primaryURL := fmt.Sprintf("%s://%s%s", cfg.Scheme, host, portSuffix)
		found := false
		for _, a := range diag.ActiveAddresses {
			if a == primaryURL {
				found = true
				break
			}
		}
		if !found {
			diag.ActiveAddresses = append([]string{primaryURL}, diag.ActiveAddresses...)
		}
	}

	// 3. Verify Hostname Resolution (mDNS)
	if cfg.MDNSServer != nil && cfg.MDNSServer.IsRunning() {
		// Run verification check
		verified, vErr := mdns.VerifyResolution(ctx, cfg.Hostname, lanIPs, 1*time.Second)
		if verified {
			diag.HostnameVerified = true
			diag.Items = append(diag.Items, DiagnosticItem{
				Category: "hostname",
				Status:   StatusOK,
				Message:  fmt.Sprintf("Hostname %s verified and resolving on local network", cfg.Hostname),
			})
			// Add verified hostname address as preferred primary address
			hostnameURL := fmt.Sprintf("%s://%s%s", cfg.Scheme, cfg.Hostname, portSuffix)
			diag.ActiveAddresses = append([]string{hostnameURL}, diag.ActiveAddresses...)
		} else {
			diag.HostnameVerified = false
			diag.Items = append(diag.Items, DiagnosticItem{
				Category: "hostname",
				Status:   StatusWarning,
				Message:  fmt.Sprintf("mDNS responder is active, but resolution of %s could not be verified locally (%v)", cfg.Hostname, vErr),
				Action:   "Check if firewall allows UDP 5353, or connect using direct LAN IP.",
			})
		}
	} else {
		diag.HostnameVerified = false
		diag.Items = append(diag.Items, DiagnosticItem{
			Category: "hostname",
			Status:   StatusWarning,
			Message:  fmt.Sprintf("mDNS responder is not active for %s", cfg.Hostname),
			Action:   "Devices must access via direct IP or add entry to /etc/hosts.",
		})
	}

	return diag
}
