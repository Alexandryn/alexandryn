package main

import (
	"fmt"
	"net"

	"github.com/Alexandryn/alexandryn/internal/config"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

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

		host, port, err := net.SplitHostPort(bindAddr)
		var addresses []transporthttp.NetworkAddressWire
		if err == nil {
			portSuffix := ":" + port
			if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
				portSuffix = ""
			}
			if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" {
				addresses = append(addresses, transporthttp.NetworkAddressWire{
					Scope: reachability,
					URL:   fmt.Sprintf("%s://%s%s", scheme, host, portSuffix),
				})
			}
			addresses = append(addresses, transporthttp.NetworkAddressWire{
				Scope: "lan",
				URL:   fmt.Sprintf("%s://alexandryn.local%s", scheme, portSuffix),
			})

			if host == "0.0.0.0" || host == "::" || host == "[::]" || host == "" {
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
								ipStr := ip.String()
								if ip.To4() == nil {
									ipStr = "[" + ipStr + "]"
								}
								scope := "lan"
								if !ip.IsPrivate() {
									scope = "public"
								}
								addresses = append(addresses, transporthttp.NetworkAddressWire{
									Scope: scope,
									URL:   fmt.Sprintf("%s://%s%s", scheme, ipStr, portSuffix),
								})
							}
						}
					}
				}
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

	host, port, err := net.SplitHostPort(cfg.BindAddress)
	if err == nil {
		portSuffix := ":" + port
		if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
			portSuffix = ""
		}
		if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" {
			origins = append(origins, fmt.Sprintf("%s://%s%s", scheme, host, portSuffix))
		}
		origins = append(origins, fmt.Sprintf("%s://alexandryn.local%s", scheme, portSuffix))

		if host == "0.0.0.0" || host == "::" || host == "[::]" || host == "" {
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
							ipStr := ip.String()
							if ip.To4() == nil {
								ipStr = "[" + ipStr + "]"
							}
							origins = append(origins, fmt.Sprintf("%s://%s%s", scheme, ipStr, portSuffix))
						}
					}
				}
			}
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
