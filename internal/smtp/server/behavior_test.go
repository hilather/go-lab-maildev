package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtptest"
	"github.com/hilather/go-lab-maildev/internal/snapshot"
)

func TestDropOnConnect(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.DropOnConnect = true
	srv := startServer(t, spec, nil)
	c, err := smtptest.Connect(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, _, err := c.ReadReply(); err == nil {
		t.Fatal("dropOnConnect must close before greeting")
	}
}

func TestGreeting421(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.Greeting = "421 4.3.2 try later"
	spec.Behavior.CloseAfterVerb = "GREETING"
	srv := startServer(t, spec, nil)
	c, err := smtptest.Connect(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	code, lines, err := c.ReadReply()
	if err != nil || code != 421 {
		t.Fatalf("code=%d %v %v", code, lines, err)
	}
	if !strings.Contains(strings.Join(lines, " "), "try later") {
		t.Fatalf("lines=%v", lines)
	}
	if _, _, err := c.Cmd("NOOP"); err == nil {
		t.Fatal("session should close after greeting 421")
	}
}

func TestMail421DoesNotAdvanceState(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.Mail = "421 4.3.2 try later"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 421, "MAIL FROM:<a@b>")
	mustCmd(t, c, 503, "RCPT TO:<c@d>")
}

func TestCloseAfterEHLO(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.CloseAfterVerb = "EHLO"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	if _, _, err := c.Cmd("NOOP"); err == nil {
		t.Fatal("session should close after EHLO")
	}
}

func TestEHLOOverrideKeepsExtensions(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.Ehlo = "250 odd.example"
	c := dial(t, startServer(t, spec, nil))
	code, lines, err := c.Cmd("EHLO x")
	if err != nil || code != 250 {
		t.Fatalf("code=%d %v %v", code, lines, err)
	}
	text := smtptest.ReplyText(lines)
	if !strings.Contains(text, "odd.example") {
		t.Fatalf("missing override first line: %v", lines)
	}
	if !strings.Contains(text, "SIZE ") {
		t.Fatalf("EHLO override must keep extension lines: %v", lines)
	}
}

func TestDataEndRejectDoesNotStore(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.DataEnd = "552 5.3.4 rejected by QA"
	sink := &stubSink{epoch: 1}
	c := dial(t, startServer(t, spec, sink))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("Subject: no"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, lines, err := c.ReadReply()
	if err != nil || code != 552 {
		t.Fatalf("code=%d %v %v", code, lines, err)
	}
	if sink.last != nil {
		t.Fatal("DATA-END 4xx/5xx must not Insert")
	}
}

func TestGreetingDelay(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.GreetingDelay = 80 * time.Millisecond
	srv := startServer(t, spec, nil)
	start := time.Now()
	c, err := smtptest.Connect(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	code, _, err := c.ReadReply()
	elapsed := time.Since(start)
	if err != nil || code != 220 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if elapsed < 60*time.Millisecond {
		t.Fatalf("greeting returned in %s; want delay", elapsed)
	}
}

func TestAuthOverrideSkipsCredentials(t *testing.T) {
	spec := authSpec(t, "lab", "secret")
	spec.Behavior.Replies.Auth = "535 5.7.8 Authentication failed"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 535, "AUTH PLAIN AGxhYgBzZWNyZXQ=")
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
}

func TestStartTLSOverride454(t *testing.T) {
	spec := starttlsSpec(t, false)
	spec.Behavior.Replies.StartTLS = "454 4.7.0 TLS not available"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 454, "STARTTLS")
	mustCmd(t, c, 250, "NOOP")
}

func TestSwapSpecBehaviorAppliesToNextMAIL(t *testing.T) {
	spec := defaultSMTPSpec(t)
	srv := startServer(t, spec, nil)
	c := dial(t, srv)
	mustCmd(t, c, 250, "EHLO x")
	next := spec
	next.Behavior.Replies.Mail = "421 4.3.2 try later"
	if err := srv.SwapSpec(next); err != nil {
		t.Fatal(err)
	}
	mustCmd(t, c, 421, "MAIL FROM:<a@b>")
	mustCmd(t, c, 503, "RCPT TO:<c@d>")
}

func TestSnapshotBehaviorAppliesToNextCommand(t *testing.T) {
	spec := defaultSMTPSpec(t)
	snaps := snapshot.NewStore()
	snaps.InstallBootstrap(&snapshot.Snapshot{
		Canonical: &model.State{Spec: model.Spec{SMTP: spec}},
	})
	srv, err := New(Options{Address: "127.0.0.1:0", Spec: spec, Snapshots: snaps})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	c := dial(t, srv)
	mustCmd(t, c, 250, "EHLO x")
	next := spec
	next.Behavior.Replies.Mail = "421 4.3.2 try later"
	snaps.Swap(&snapshot.Snapshot{
		Canonical: &model.State{Spec: model.Spec{SMTP: next}},
	})
	mustCmd(t, c, 421, "MAIL FROM:<a@b>")
}

func TestData421DoesNotEnterBody(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.Data = "421 4.3.2 try later"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 421, "DATA")
	mustCmd(t, c, 250, "NOOP")
}

func TestMailOverrideDoesNotWinBeforeEHLO(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.Mail = "421 4.3.2 try later"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 503, "MAIL FROM:<a@b>")
}

func TestCommandDelay(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.CommandDelay = 80 * time.Millisecond
	c := dial(t, startServer(t, spec, nil))
	start := time.Now()
	mustCmd(t, c, 250, "NOOP")
	if elapsed := time.Since(start); elapsed < 60*time.Millisecond {
		t.Fatalf("NOOP returned in %s; want commandDelay", elapsed)
	}
}

func TestRsetErrorOverrideKeepsTransaction(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.Rset = "550 5.7.1 no reset"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 550, "RSET")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, _, err := c.ReadReply()
	if err != nil || code != 250 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestUnknownOverride(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Behavior.Replies.Unknown = "502 5.5.1 no such verb"
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 502, "FROBNITZ")
}
