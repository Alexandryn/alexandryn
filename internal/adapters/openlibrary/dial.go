package openlibrary

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// maxRedirectHops caps a single request's redirect chain. Open Library
// uses one hop (covers.openlibrary.org -> archive.org); anything longer
// is treated as abnormal.
const maxRedirectHops = 5

// cgnatPrefix is RFC 6598 carrier-grade NAT space (100.64.0.0/10).
var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// blockedResolvedIP reports whether ip is one an Open Library request
// must never reach. Open Library is a fixed public service, so — unlike
// the user-configured source client's sources.IsBlockedDialIP, whose
// sibling this mirrors (#251) — there is no allow-private opt-out:
// loopback, RFC 1918 / IPv6-ULA private, link-local in its plain form
// (the 169.254.169.254 cloud-metadata address), CGNAT, multicast and the
// unspecified address are all refused.
//
// Not decoded: an internal IPv4 embedded in a transitional IPv6 form
// (NAT64, 6to4, Teredo) or the reserved 240.0.0.0/4 / broadcast ranges —
// same gap and same follow-up as sources.IsBlockedDialIP.
func blockedResolvedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if addr, ok := netip.AddrFromSlice(ip); ok && cgnatPrefix.Contains(addr.Unmap()) {
		return true
	}
	return false
}

// blockedAddressError is returned by the guarded dialer.
type blockedAddressError struct{ addr string }

func (e *blockedAddressError) Error() string {
	return fmt.Sprintf("openlibrary: refusing to connect to non-public address %s", e.addr)
}

// guardedTransport is an *http.Transport whose dialer refuses any
// connection whose resolved peer is non-public. The check runs after DNS
// resolution against the concrete IP, so it also closes the
// DNS-rebinding and redirect-to-internal gaps (#251).
func guardedTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return &blockedAddressError{addr: address}
			}
			ip := net.ParseIP(host)
			if ip == nil || blockedResolvedIP(ip) {
				return &blockedAddressError{addr: host}
			}
			return nil
		},
	}
	return &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// capRedirects limits a request's redirect chain to maxRedirectHops. The
// per-hop address check is the transport's job; this only bounds the
// chain length.
func capRedirects(_ *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirectHops {
		return fmt.Errorf("openlibrary: stopped after %d redirects", maxRedirectHops)
	}
	return nil
}
