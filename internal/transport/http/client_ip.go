package http

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
)

// trustedProxyCIDRs holds the operator-configured reverse-proxy networks
// (config.TrustedProxyCIDRs, #195). nil / empty means no proxy is
// trusted: clientIP then keys purely on the connection's RemoteAddr and
// ignores any X-Forwarded-For.
var trustedProxyCIDRs atomic.Pointer[[]netip.Prefix]

// SetTrustedProxyCIDRs installs the trusted reverse-proxy ranges. Called
// once at startup from the resolved config.
func SetTrustedProxyCIDRs(prefixes []netip.Prefix) {
	cp := append([]netip.Prefix(nil), prefixes...)
	trustedProxyCIDRs.Store(&cp)
}

// clientIP returns the rate-limiting key for r. The key is the client's
// network, not its exact address: an IPv6 address collapses to its /64
// (a single allocation an attacker rotates for free), an IPv4 address
// stays as itself. When RemoteAddr is a configured trusted proxy, the
// client address is taken from the last X-Forwarded-For hop; otherwise
// it is RemoteAddr, and a client-supplied X-Forwarded-For is ignored.
func clientIP(r *http.Request) string {
	host := hostOnly(r.RemoteAddr)
	addr, err := netip.ParseAddr(host)
	if err != nil {
		// Unparseable RemoteAddr (e.g. a test's bare string): key on it
		// verbatim rather than silently collapsing distinct callers.
		return host
	}

	if prefixes := trustedProxyCIDRs.Load(); prefixes != nil && addrInAny(addr, *prefixes) {
		if hop, ok := lastForwardedHop(r.Header.Get("X-Forwarded-For")); ok {
			addr = hop
		}
	}

	return rateLimitKey(addr)
}

// requestIsHTTPS reports whether the client reached this server over
// TLS: either this process terminated it, or a configured trusted proxy
// did and said so in X-Forwarded-Proto. A client-supplied
// X-Forwarded-Proto from anywhere else is ignored.
func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	addr, err := netip.ParseAddr(hostOnly(r.RemoteAddr))
	if err != nil {
		return false
	}
	prefixes := trustedProxyCIDRs.Load()
	if prefixes == nil || !addrInAny(addr, *prefixes) {
		return false
	}
	// Chained proxies append: "https, http" means the client-facing hop was
	// TLS, so only the first (client-most) entry counts. That entry may be
	// client-supplied when a trusted proxy appends rather than overwrites,
	// but it only decides the Secure flag on the cookie set in the response
	// to that same client: a client lying here can only mislabel its own
	// cookie, never anyone else's.
	first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(first), "https")
}

// hostOnly strips a trailing :port from a host:port string, tolerating
// bare hosts and bracketed IPv6 literals.
func hostOnly(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return strings.Trim(remoteAddr, "[]")
}

func addrInAny(a netip.Addr, prefixes []netip.Prefix) bool {
	a = a.Unmap()
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// lastForwardedHop returns the right-most address in an X-Forwarded-For
// header — the hop the trusted proxy itself observed, the only entry a
// downstream client cannot forge past that proxy.
func lastForwardedHop(xff string) (netip.Addr, bool) {
	if xff == "" {
		return netip.Addr{}, false
	}
	parts := strings.Split(xff, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	addr, err := netip.ParseAddr(last)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// rateLimitKey reduces an address to the network the limiter buckets on.
func rateLimitKey(a netip.Addr) string {
	a = a.Unmap()
	if a.Is6() {
		if p, err := a.Prefix(64); err == nil {
			return p.String()
		}
	}
	return a.String()
}
