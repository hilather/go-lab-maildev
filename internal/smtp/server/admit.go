package server

import (
	"errors"
	"net"
	"net/netip"
	"sync"

	"github.com/hilather/go-lab-maildev/internal/model"
)

var (
	errTooManySessions = errors.New("too many sessions")
	errTooManyInFlight = errors.New("too many in-flight DATA")
	errInFlightBytes   = errors.New("in-flight DATA budget exceeded")
)

type gate struct {
	mu       sync.Mutex
	sessions int
	perIP    map[netip.Addr]int
	inflight int
	reserved int64
}

func newGate() *gate {
	return &gate{perIP: make(map[netip.Addr]int)}
}

func (g *gate) acquire(ip netip.Addr, ad model.AdmissionSpec) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if ad.MaxSessions > 0 && g.sessions >= ad.MaxSessions {
		return errTooManySessions
	}
	if ip.IsValid() && ad.MaxSessionsPerIP > 0 && g.perIP[ip] >= ad.MaxSessionsPerIP {
		return errTooManySessions
	}
	g.sessions++
	if ip.IsValid() {
		g.perIP[ip]++
	}
	return nil
}

func (g *gate) release(ip netip.Addr) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.sessions > 0 {
		g.sessions--
	}
	if ip.IsValid() {
		n := g.perIP[ip] - 1
		if n <= 0 {
			delete(g.perIP, ip)
		} else {
			g.perIP[ip] = n
		}
	}
}

func (g *gate) reserveData(n int64, ad model.AdmissionSpec) error {
	if n < 0 {
		n = 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if ad.MaxInFlightData > 0 && g.inflight >= ad.MaxInFlightData {
		return errTooManyInFlight
	}
	if ad.MaxInFlightDataBytes > 0 && g.reserved+n > ad.MaxInFlightDataBytes {
		return errInFlightBytes
	}
	g.inflight++
	g.reserved += n
	return nil
}

func (g *gate) releaseData(n int64) {
	if n < 0 {
		n = 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inflight > 0 {
		g.inflight--
	}
	g.reserved -= n
	if g.reserved < 0 {
		g.reserved = 0
	}
}

func remoteIP(a net.Addr) netip.Addr {
	if a == nil {
		return netip.Addr{}
	}
	if ta, ok := a.(*net.TCPAddr); ok && ta.IP != nil {
		ip, ok := netip.AddrFromSlice(ta.IP)
		if !ok {
			return netip.Addr{}
		}
		return ip.Unmap()
	}
	host, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return netip.Addr{}
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}
