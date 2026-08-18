package server

import (
	"net"
	"net/netip"
	"syscall"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestGateAcquireRelease(t *testing.T) {
	g := newGate()
	ad := model.AdmissionSpec{MaxSessions: 2, MaxSessionsPerIP: 1}
	ip := netip.MustParseAddr("127.0.0.1")
	if err := g.acquire(ip, ad); err != nil {
		t.Fatal(err)
	}
	if err := g.acquire(ip, ad); err == nil {
		t.Fatal("per-ip should refuse")
	}
	other := netip.MustParseAddr("10.0.0.2")
	if err := g.acquire(other, ad); err != nil {
		t.Fatal(err)
	}
	if err := g.acquire(netip.MustParseAddr("10.0.0.3"), ad); err == nil {
		t.Fatal("max sessions should refuse")
	}
	g.release(ip)
	if err := g.acquire(ip, ad); err != nil {
		t.Fatal(err)
	}
}

func TestGateReserveData(t *testing.T) {
	g := newGate()
	ad := model.AdmissionSpec{MaxInFlightData: 1, MaxInFlightDataBytes: 100}
	if err := g.reserveData(80, ad); err != nil {
		t.Fatal(err)
	}
	if err := g.reserveData(10, ad); err == nil {
		t.Fatal("count cap")
	}
	g.releaseData(80)
	if err := g.reserveData(101, ad); err == nil {
		t.Fatal("byte cap")
	}
	if err := g.reserveData(100, ad); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteIP(t *testing.T) {
	ta := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9}
	ip := remoteIP(ta)
	if !ip.Is4() || ip.String() != "127.0.0.1" {
		t.Fatalf("%v", ip)
	}
	mapped := &net.TCPAddr{IP: net.ParseIP("::ffff:10.1.2.3"), Port: 1}
	ip = remoteIP(mapped)
	if ip.String() != "10.1.2.3" {
		t.Fatalf("unmap %v", ip)
	}
}

func TestAcceptShouldStop(t *testing.T) {
	if !acceptShouldStop(net.ErrClosed) {
		t.Fatal("closed listener must stop")
	}
	if acceptShouldStop(syscall.EMFILE) || acceptShouldStop(syscall.ENFILE) {
		t.Fatal("EMFILE/ENFILE must keep accepting")
	}
	if acceptShouldStop(nil) {
		t.Fatal("nil")
	}
}
