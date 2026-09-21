package sources

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// cgnatPrefix is RFC 6598 carrier-grade NAT space (100.64.0.0/10). net.IP
// carries no predicate for it, so it is checked explicitly.
var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// IsBlockedDialIP reports whether an outbound source request must never
// reach ip (a source base URL is attacker-chosen input).
//
// Always blocked, no opt-out: link-local in its plain form (the
// 169.254.169.254 cloud-metadata address and fe80::/10), carrier-grade
// NAT (100.64.0.0/10), multicast, and the unspecified address — none of
// these is ever a legitimate OPDS host.
//
// Not decoded: an internal IPv4 embedded in a transitional IPv6 form
// (NAT64 64:ff9b::/96, 6to4 2002::/16, Teredo 2001::/32), or the
// reserved 240.0.0.0/4 / broadcast / 0.0.0.0/8-with-host ranges. Go's
// net predicates only unwrap the IPv4-mapped form. The practical path to
// these is narrow (RFC 6052 forbids a compliant NAT64 translating a
// special-use address; 6to4/Teredo are effectively dead) — tracked for a
// follow-up that adds explicit prefix checks.
//
// Blocked unless allowPrivate is set: loopback and RFC 1918 / IPv6-ULA
// private ranges. A self-hoster running a source on the same machine
// (localhost:8083 Calibre-web) or elsewhere on their LAN opts in with
// SOURCE_ALLOW_PRIVATE_ADDRESSES.
func IsBlockedDialIP(ip net.IP, allowPrivate bool) bool {
	if ip == nil {
		return true
	}
	if ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if addr, ok := netip.AddrFromSlice(ip); ok && cgnatPrefix.Contains(addr.Unmap()) {
		return true
	}
	if !allowPrivate && (ip.IsLoopback() || ip.IsPrivate()) {
		return true
	}
	return false
}

// RejectBlockedLiteralHost fast-fails a base URL whose host is an IP
// literal in a blocked range, so an obvious mistake is a clear
// create-time InvalidInput rather than a later "unreachable" health
// check. A hostname is left for the dial-time guard (which also handles
// DNS rebinding); this only catches literals.
func RejectBlockedLiteralHost(baseURL string, allowPrivate bool) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil // shape is ValidateOPDSBaseURL's job
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		return nil
	}
	if IsBlockedDialIP(ip, allowPrivate) {
		return &domain.Error{
			Category: domain.InvalidInput,
			Message:  "config.baseUrl points at a non-public address; set SOURCE_ALLOW_PRIVATE_ADDRESSES to allow a source on this machine or your LAN",
		}
	}
	return nil
}

// BlockedAddressError is returned when the guarded dialer refuses a
// connection to a non-permitted address.
type BlockedAddressError struct{ Addr string }

func (e *BlockedAddressError) Error() string {
	return fmt.Sprintf("address %s is not a permitted source address", e.Addr)
}

// GuardedTransport is an *http.Transport that refuses to open a
// connection whose resolved peer is in a blocked range. The check runs in
// the dialer Control hook, after DNS resolution, against the concrete IP
// about to be dialed — so it also closes the DNS-rebinding gap that a
// hostname check alone would leave open. Redirect handling is the
// caller's concern (the OPDS client disables redirects entirely).
func GuardedTransport(allowPrivate bool) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return &BlockedAddressError{Addr: address}
			}
			ip := net.ParseIP(host)
			if ip == nil {
				// Control is called post-resolution; a non-literal here
				// means something is wrong. Fail closed.
				return &BlockedAddressError{Addr: host}
			}
			if IsBlockedDialIP(ip, allowPrivate) {
				return &BlockedAddressError{Addr: host}
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
