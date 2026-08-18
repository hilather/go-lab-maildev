package server

import (
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/smtptest"
	"github.com/hilather/go-lab-maildev/internal/store"
)

func TestSIZERejectAtMAIL(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.MaxMessageBytes = 100
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 552, "MAIL FROM:<a@b> SIZE=200")
}

func TestDATATooLarge(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.MaxMessageBytes = 40
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine(strings.Repeat("z", 80)); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, lines, err := c.ReadReply()
	if err != nil || code != 552 {
		t.Fatalf("code=%d %v %v", code, lines, err)
	}
}

func TestRecipientCap(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.MaxRecipients = 2
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<one@lab.test>")
	mustCmd(t, c, 250, "RCPT TO:<two@lab.test>")
	mustCmd(t, c, 452, "RCPT TO:<three@lab.test>")
}

func TestSessionCap(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Admission.MaxSessions = 1
	spec.Admission.MaxSessionsPerIP = 32
	srv := startServer(t, spec, nil)
	c1 := dial(t, srv)
	c2, err := smtptest.Connect(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()
	code, _, err := c2.ReadReply()
	if err != nil {
		t.Fatal(err)
	}
	if code != 421 {
		t.Fatalf("second session code=%d", code)
	}
	_ = c1
}

func TestInFlightDataCap(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Admission.MaxInFlightData = 1
	spec.Admission.MaxInFlightDataBytes = 64 << 20
	srv := startServer(t, spec, nil)
	c1 := dial(t, srv)
	mustCmd(t, c1, 250, "EHLO x")
	mustCmd(t, c1, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c1, 250, "RCPT TO:<c@d>")
	mustCmd(t, c1, 354, "DATA")

	c2 := dial(t, srv)
	mustCmd(t, c2, 250, "EHLO y")
	mustCmd(t, c2, 250, "MAIL FROM:<e@f>")
	mustCmd(t, c2, 250, "RCPT TO:<g@h>")
	mustCmd(t, c2, 421, "DATA")

	if err := c1.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c1.ReadReply(); err != nil {
		t.Fatal(err)
	}
	mustCmd(t, c2, 354, "DATA")
	if err := c2.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c2.ReadReply(); err != nil {
		t.Fatal(err)
	}
}

func TestInFlightByteReserve(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.MaxMessageBytes = 1000
	spec.Admission.MaxInFlightData = 8
	spec.Admission.MaxInFlightDataBytes = 1000
	srv := startServer(t, spec, nil)
	c1 := dial(t, srv)
	mustCmd(t, c1, 250, "EHLO x")
	mustCmd(t, c1, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c1, 250, "RCPT TO:<c@d>")
	mustCmd(t, c1, 354, "DATA")

	c2 := dial(t, srv)
	mustCmd(t, c2, 250, "EHLO y")
	mustCmd(t, c2, 250, "MAIL FROM:<e@f>")
	mustCmd(t, c2, 250, "RCPT TO:<g@h>")
	mustCmd(t, c2, 452, "DATA")

	_ = c1.WriteLine(".")
	_, _, _ = c1.ReadReply()
}

func TestStaleEpochAbortsDATA(t *testing.T) {
	sink := store.NewNull()
	spec := defaultSMTPSpec(t)
	srv := startServer(t, spec, sink)
	c := dial(t, srv)
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	sink.Wipe()
	if err := c.WriteLine("Subject: gone"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, lines, err := c.ReadReply()
	if err != nil || code != 451 {
		t.Fatalf("code=%d %v %v", code, lines, err)
	}
}

func TestSwapSpecAppliesToNextMAIL(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.MaxMessageBytes = 10 << 20
	srv := startServer(t, spec, nil)
	c := dial(t, srv)
	mustCmd(t, c, 250, "EHLO x")
	next := spec
	next.MaxMessageBytes = 50
	srv.SwapSpec(next)
	mustCmd(t, c, 552, "MAIL FROM:<a@b> SIZE=100")
}

func TestCommandIdleTimeout(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Admission.CommandIdle = 80 * time.Millisecond
	spec.Admission.SessionTimeout = time.Second
	c := dial(t, startServer(t, spec, nil))
	time.Sleep(200 * time.Millisecond)
	_, _, err := c.Cmd("NOOP")
	if err == nil {
		t.Fatal("expected idle close")
	}
}

func TestDATAIdleTimeout(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Admission.DataIdle = 80 * time.Millisecond
	spec.Admission.CommandIdle = time.Second
	spec.Admission.SessionTimeout = 2 * time.Second
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	time.Sleep(200 * time.Millisecond)
	code, _, err := c.ReadReply()
	if err == nil && code != 451 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}
