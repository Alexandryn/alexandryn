package mdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// VerifyResolution tests whether the given hostname (e.g. alexandryn.local) resolves
// to an expected IP address, checking both the OS resolver and direct mDNS multicast query.
func VerifyResolution(ctx context.Context, hostname string, expectedIPs []net.IP, timeout time.Duration) (bool, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	vCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cleanHost := strings.TrimSuffix(hostname, ".")

	// 1. Try standard system resolver
	ips, err := net.DefaultResolver.LookupHost(vCtx, cleanHost)
	if err == nil && len(ips) > 0 {
		if len(expectedIPs) == 0 {
			return true, nil
		}
		for _, ipStr := range ips {
			parsed := net.ParseIP(ipStr)
			if parsed != nil {
				for _, exp := range expectedIPs {
					if parsed.Equal(exp) {
						return true, nil
					}
				}
			}
		}
	}

	// 2. Try direct mDNS query over UDP 5353
	verified, mDNSErr := queryMDNSDirect(vCtx, cleanHost, expectedIPs)
	if verified {
		return true, nil
	}

	if mDNSErr != nil {
		return false, fmt.Errorf("hostname %q could not be resolved: %w", hostname, mDNSErr)
	}

	return false, fmt.Errorf("hostname %q did not resolve to expected LAN IP", hostname)
}

func queryMDNSDirect(ctx context.Context, hostname string, expectedIPs []net.IP) (bool, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return false, err
	}
	defer func() { _ = conn.Close() }()

	targetFQDN := hostname + "."
	name, err := dnsmessage.NewName(targetFQDN)
	if err != nil {
		return false, err
	}

	query := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               1234,
			RecursionDesired: false,
		},
		Questions: []dnsmessage.Question{
			{
				Name:  name,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
			},
		},
	}

	packed, err := query.Pack()
	if err != nil {
		return false, err
	}

	destAddr, err := net.ResolveUDPAddr("udp4", mdnsIPv4Addr)
	if err != nil {
		return false, err
	}

	if _, err := conn.WriteTo(packed, destAddr); err != nil {
		return false, err
	}

	buf := make([]byte, 1500)
	doneChan := make(chan bool, 1)

	go func() {
		for {
			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, _, readErr := conn.ReadFrom(buf)
			if readErr != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}

			var resp dnsmessage.Message
			if err := resp.Unpack(buf[:n]); err != nil || !resp.Response {
				continue
			}

			for _, ans := range resp.Answers {
				if strings.EqualFold(ans.Header.Name.String(), targetFQDN) {
					if a, ok := ans.Body.(*dnsmessage.AResource); ok {
						respIP := net.IPv4(a.A[0], a.A[1], a.A[2], a.A[3])
						if len(expectedIPs) == 0 {
							doneChan <- true
							return
						}
						for _, exp := range expectedIPs {
							if respIP.Equal(exp) {
								doneChan <- true
								return
							}
						}
					}
				}
			}
		}
	}()

	select {
	case <-doneChan:
		return true, nil
	case <-ctx.Done():
		return false, errors.New("resolution check timed out")
	}
}
