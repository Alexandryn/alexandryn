package mdns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	// Multicast IPv4 address and port for mDNS (RFC 6762).
	mdnsIPv4Addr = "224.0.0.251:5353"
	defaultTTL   = 120 // Seconds
)

// Config configures the mDNS responder.
type Config struct {
	Hostname string // e.g. "alexandryn.local"
	Port     int    // HTTP port, e.g. 80 or 8080
	IPs      []net.IP
	Logger   *slog.Logger
}

// Server is an RFC 6762 Multicast DNS responder for Alexandryn.
type Server struct {
	cfg       Config
	conn      *net.UDPConn
	logger    *slog.Logger
	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup

	mu         sync.RWMutex
	activeIPs  []net.IP
	listenErr  error
	isRunning  bool
	isVerified bool
}

// NewServer constructs an mDNS responder.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Hostname == "" {
		cfg.Hostname = "alexandryn.local"
	}
	cfg.Hostname = strings.TrimSuffix(cfg.Hostname, ".")
	if cfg.Port <= 0 {
		cfg.Port = 80
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Server{
		cfg:    cfg,
		logger: logger,
		done:   make(chan struct{}),
	}, nil
}

// discoverLANIPs enumerates all non-loopback up interfaces with private IPv4 addresses.
func discoverLANIPs() []net.IP {
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
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
				ips = append(ips, ip.To4())
			}
		}
	}
	return ips
}

// Start begins listening for multicast DNS queries on UDP 5353 and broadcasts announcements.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return nil
	}

	ips := s.cfg.IPs
	if len(ips) == 0 {
		ips = discoverLANIPs()
	}
	s.activeIPs = ips

	addr, err := net.ResolveUDPAddr("udp4", mdnsIPv4Addr)
	if err != nil {
		s.listenErr = err
		s.mu.Unlock()
		return fmt.Errorf("mdns: failed to resolve multicast address: %w", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		s.listenErr = err
		s.mu.Unlock()
		s.logger.Warn("mdns: failed to bind multicast UDP port 5353 (check permissions or existing daemon)", "error", err.Error())
		return fmt.Errorf("mdns: failed to listen on %s: %w", mdnsIPv4Addr, err)
	}

	s.conn = conn
	s.isRunning = true
	s.mu.Unlock()

	s.logger.Info("mdns responder started", "hostname", s.cfg.Hostname, "port", s.cfg.Port, "ips", len(s.activeIPs))

	// Send initial announcement
	s.broadcastAnnouncement(defaultTTL)

	s.wg.Add(1)
	go s.serveLoop()

	// Actively verify hostname resolution in the background
	go func() {
		vCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		verified, err := VerifyResolution(vCtx, s.cfg.Hostname, s.activeIPs, 2*time.Second)
		s.mu.Lock()
		s.isVerified = verified
		s.mu.Unlock()
		if verified {
			s.logger.Info("mdns: hostname resolution actively verified", "hostname", s.cfg.Hostname)
		} else {
			s.logger.Warn("mdns: hostname resolution unverified (check firewall or local mDNS daemon)", "hostname", s.cfg.Hostname, "error", err)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			_ = s.Close()
		case <-s.done:
		}
	}()

	return nil
}

// IsRunning reports whether the responder is active.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// IsVerified reports whether hostname resolution has been actively verified on the network.
func (s *Server) IsVerified() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isVerified
}

// SetVerified explicitly sets the verification status (e.g. for testing or external verification).
func (s *Server) SetVerified(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isVerified = v
}

// ActiveIPs returns the list of IPv4 addresses currently advertised.
func (s *Server) ActiveIPs() []net.IP {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]net.IP, len(s.activeIPs))
	copy(out, s.activeIPs)
	return out
}

// Close gracefully stops the responder and sends goodbye packets.
func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.isRunning = false
		s.isVerified = false
		conn := s.conn
		s.mu.Unlock()

		close(s.done)

		// Broadcast goodbye (TTL=0)
		s.broadcastAnnouncement(0)

		if conn != nil {
			err = conn.Close()
		}
		s.wg.Wait()
		s.logger.Info("mdns responder stopped")
	})
	return err
}

func (s *Server) serveLoop() {
	defer s.wg.Done()
	buf := make([]byte, 1500)

	for {
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()
		if conn == nil {
			return
		}

		n, src, err := conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				// Temporary or network error; keep going if not closed
				continue
			}
		}

		var msg dnsmessage.Message
		if err := msg.Unpack(buf[:n]); err != nil {
			continue
		}

		// Only process queries (QR == 0)
		if msg.Response {
			continue
		}

		s.handleQuery(&msg, src)
	}
}

func (s *Server) handleQuery(msg *dnsmessage.Message, src net.Addr) {
	s.mu.RLock()
	hostname := s.cfg.Hostname
	ips := s.activeIPs
	port := s.cfg.Port
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil || len(ips) == 0 {
		return
	}

	targetFQDN := hostname + "."
	httpService := "_http._tcp.local."
	instanceName := "Alexandryn._http._tcp.local."

	var answers []dnsmessage.Resource
	var additionals []dnsmessage.Resource

	for _, q := range msg.Questions {
		qName := strings.ToLower(q.Name.String())

		// Match A record for hostname (e.g. "alexandryn.local.")
		if (q.Type == dnsmessage.TypeA || q.Type == dnsmessage.TypeALL) && strings.EqualFold(qName, targetFQDN) {
			targetName, err := dnsmessage.NewName(targetFQDN)
			if err != nil {
				continue
			}
			for _, ip := range ips {
				var a [4]byte
				copy(a[:], ip.To4())
				answers = append(answers, dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{
						Name:  targetName,
						Type:  dnsmessage.TypeA,
						Class: dnsmessage.ClassINET,
						TTL:   defaultTTL,
					},
					Body: &dnsmessage.AResource{A: a},
				})
			}
		}

		// Match service discovery: PTR query for "_http._tcp.local."
		if (q.Type == dnsmessage.TypePTR || q.Type == dnsmessage.TypeALL) && strings.EqualFold(qName, httpService) {
			serviceName, err := dnsmessage.NewName(httpService)
			instName, err2 := dnsmessage.NewName(instanceName)
			targetName, err3 := dnsmessage.NewName(targetFQDN)
			if err == nil && err2 == nil && err3 == nil {
				answers = append(answers, dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{
						Name:  serviceName,
						Type:  dnsmessage.TypePTR,
						Class: dnsmessage.ClassINET,
						TTL:   defaultTTL,
					},
					Body: &dnsmessage.PTRResource{PTR: instName},
				})

				// Include SRV and A records in additionals
				additionals = append(additionals, dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{
						Name:  instName,
						Type:  dnsmessage.TypeSRV,
						Class: dnsmessage.ClassINET,
						TTL:   defaultTTL,
					},
					Body: &dnsmessage.SRVResource{
						Priority: 0,
						Weight:   0,
						Port:     uint16(port),
						Target:   targetName,
					},
				})
				for _, ip := range ips {
					var a [4]byte
					copy(a[:], ip.To4())
					additionals = append(additionals, dnsmessage.Resource{
						Header: dnsmessage.ResourceHeader{
							Name:  targetName,
							Type:  dnsmessage.TypeA,
							Class: dnsmessage.ClassINET,
							TTL:   defaultTTL,
						},
						Body: &dnsmessage.AResource{A: a},
					})
				}
			}
		}
	}

	if len(answers) == 0 {
		return
	}

	resp := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:            msg.ID,
			Response:      true,
			Authoritative: true,
		},
		Answers:     answers,
		Additionals: additionals,
	}

	packed, err := resp.Pack()
	if err != nil {
		return
	}

	// In mDNS, responses can be multicast back to 224.0.0.251:5353 or unicast to querier
	destAddr, _ := net.ResolveUDPAddr("udp4", mdnsIPv4Addr)
	if udpSrc, ok := src.(*net.UDPAddr); ok && udpSrc.Port != 5353 {
		// Standard legacy unicast DNS query on non-5353 source port
		_, _ = conn.WriteTo(packed, src)
	} else {
		_, _ = conn.WriteTo(packed, destAddr)
	}
}

func (s *Server) broadcastAnnouncement(ttl uint32) {
	s.mu.RLock()
	hostname := s.cfg.Hostname
	ips := s.activeIPs
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil || len(ips) == 0 {
		return
	}

	targetFQDN := hostname + "."
	targetName, err := dnsmessage.NewName(targetFQDN)
	if err != nil {
		return
	}

	var answers []dnsmessage.Resource
	for _, ip := range ips {
		var a [4]byte
		copy(a[:], ip.To4())
		answers = append(answers, dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{
				Name:  targetName,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
				TTL:   ttl,
			},
			Body: &dnsmessage.AResource{A: a},
		})
	}

	msg := dnsmessage.Message{
		Header: dnsmessage.Header{
			Response:      true,
			Authoritative: true,
		},
		Answers: answers,
	}

	packed, err := msg.Pack()
	if err != nil {
		return
	}

	destAddr, err := net.ResolveUDPAddr("udp4", mdnsIPv4Addr)
	if err == nil {
		_, _ = conn.WriteTo(packed, destAddr)
	}
}
