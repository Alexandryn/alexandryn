package main

import (
	"fmt"
	"net"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// resolvedBindAddress is the shared address-derivation result for a
// configured bind address. fallbackNetworkInfo (network-status display),
// computeAllowedOrigins (CORS/Origin allowlist), and resolveServerAddress
// (pairing QR/deep-link payload) each format their own output from this,
// so a fix to the derivation (e.g. the wildcard-host or link-local-scope
// bugs this type replaces two independent copies of) can't silently
// diverge between them again.
type resolvedBindAddress struct {
	port        string
	primaryHost string   // "" if the configured bind is a wildcard host
	extraHosts  []net.IP // non-loopback interface addresses; populated only when the bind is a wildcard
}

// resolveBindAddress splits bindAddr and, if its host is a wildcard
// (0.0.0.0, ::, or [::]), enumerates every up, non-loopback interface
// address as the dialable candidates instead. ok is false only when
// bindAddr itself doesn't parse as host:port.
func resolveBindAddress(bindAddr string) (resolvedBindAddress, bool) {
	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return resolvedBindAddress{}, false
	}
	r := resolvedBindAddress{port: port}
	if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" {
		r.primaryHost = host
		return r, true
	}

	if ifaces, err := net.Interfaces(); err == nil {
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
				if ip != nil && !ip.IsLoopback() {
					r.extraHosts = append(r.extraHosts, ip)
				}
			}
		}
	}
	return r, true
}

// scopeForIP classifies an interface address for the network-status
// display. Link-local (169.254.0.0/16, fe80::/10) is never publicly
// routable — Go's net.IP.IsPrivate() does not cover it, so it must be
// checked separately or it falls through to "public".
func scopeForIP(ip net.IP) string {
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return "lan"
	}
	return "public"
}

// formatHost renders an interface IP for use in a URL authority,
// bracketing it if it's IPv6.
func formatHost(ip net.IP) string {
	s := ip.String()
	if ip.To4() == nil {
		return "[" + s + "]"
	}
	return s
}

func fallbackNetworkInfo(cfg *config.Config) func() transporthttp.NetworkInfo {
	return func() transporthttp.NetworkInfo {
		scheme := "http"
		if cfg != nil && (cfg.TLSMode() == "static" || cfg.TLSMode() == "acme") {
			scheme = "https"
		}

		reachability := "lan"
		bindAddr := "127.0.0.1:0"
		tlsMode := "none"
		acmeDomain := ""
		if cfg != nil {
			if cfg.Reachability() != "" {
				reachability = cfg.Reachability()
			}
			if cfg.BindAddress != "" {
				bindAddr = cfg.BindAddress
			}
			if cfg.TLSMode() != "" {
				tlsMode = cfg.TLSMode()
			}
			acmeDomain = cfg.ACMEDomain
		}

		var addresses []transporthttp.NetworkAddressWire
		if resolved, ok := resolveBindAddress(bindAddr); ok {
			portSuffix := ":" + resolved.port
			if (scheme == "http" && resolved.port == "80") || (scheme == "https" && resolved.port == "443") {
				portSuffix = ""
			}
			if resolved.primaryHost != "" {
				addresses = append(addresses, transporthttp.NetworkAddressWire{
					Scope: reachability,
					URL:   fmt.Sprintf("%s://%s%s", scheme, resolved.primaryHost, portSuffix),
				})
			}
			addresses = append(addresses, transporthttp.NetworkAddressWire{
				Scope: "lan",
				URL:   fmt.Sprintf("%s://alexandryn.local%s", scheme, portSuffix),
			})
			for _, ip := range resolved.extraHosts {
				addresses = append(addresses, transporthttp.NetworkAddressWire{
					Scope: scopeForIP(ip),
					URL:   fmt.Sprintf("%s://%s%s", scheme, formatHost(ip), portSuffix),
				})
			}
		}

		return transporthttp.NetworkInfo{
			Reachability: reachability,
			TLSMode:      tlsMode,
			BindAddress:  bindAddr,
			ACMEDomain:   acmeDomain,
			HostName:     "alexandryn.local",
			Addresses:    addresses,
		}
	}
}

func computeAllowedOrigins(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	origins := make([]string, 0, len(cfg.CORSAllowedOrigins)+4)
	origins = append(origins, cfg.CORSAllowedOrigins...)

	scheme := "http"
	if cfg.TLSMode() == "static" || cfg.TLSMode() == "acme" {
		scheme = "https"
	}

	if resolved, ok := resolveBindAddress(cfg.BindAddress); ok {
		portSuffix := ":" + resolved.port
		if (scheme == "http" && resolved.port == "80") || (scheme == "https" && resolved.port == "443") {
			portSuffix = ""
		}
		if resolved.primaryHost != "" {
			origins = append(origins, fmt.Sprintf("%s://%s%s", scheme, resolved.primaryHost, portSuffix))
		}
		origins = append(origins, fmt.Sprintf("%s://alexandryn.local%s", scheme, portSuffix))
		for _, ip := range resolved.extraHosts {
			origins = append(origins, fmt.Sprintf("%s://%s%s", scheme, formatHost(ip), portSuffix))
		}
	}

	seen := make(map[string]bool)
	var deduped []string
	for _, o := range origins {
		if !seen[o] && o != "" {
			seen[o] = true
			deduped = append(deduped, o)
		}
	}
	return deduped
}

// resolveServerAddress returns a dialable host:port for the pairing
// QR/deep-link payload and the network-status Address field. Unlike a raw
// cfg.BindAddress, it never returns a wildcard host (0.0.0.0, ::, [::]) —
// undialable from another device, exactly the reachability class this
// feature targets — falling back to the first non-loopback interface
// address (preferring a LAN/private one) the same way fallbackNetworkInfo
// already resolves addresses for display.
func resolveServerAddress(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	resolved, ok := resolveBindAddress(cfg.BindAddress)
	if !ok {
		return cfg.BindAddress
	}
	if resolved.primaryHost != "" {
		return net.JoinHostPort(resolved.primaryHost, resolved.port)
	}
	for _, ip := range resolved.extraHosts {
		if scopeForIP(ip) == "lan" {
			return net.JoinHostPort(ip.String(), resolved.port)
		}
	}
	if len(resolved.extraHosts) > 0 {
		return net.JoinHostPort(resolved.extraHosts[0].String(), resolved.port)
	}
	// A wildcard bind with no discoverable non-loopback interface: no
	// dialable address is known. Surfacing the undialable wildcard host
	// would be worse than an empty string.
	return ""
}
