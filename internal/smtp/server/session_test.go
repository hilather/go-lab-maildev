package server

import (
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtptest"
)

func TestGreetingHasNoMaildev(t *testing.T) {
	srv := startServer(t, model.SMTPSpec{}, nil)
	c, err := smtptest.Connect(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	code, lines, err := c.ReadReply()
	if err != nil {
		t.Fatal(err)
	}
	if code != 220 {
		t.Fatalf("code=%d", code)
	}
	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "LabMail ready") {
		t.Fatalf("greeting %q", joined)
	}
	if strings.Contains(strings.ToLower(joined), "maildev") {
		t.Fatalf("banner must not contain maildev: %q", joined)
	}
}

func TestEHLOExtensions(t *testing.T) {
	c := dial(t, startServer(t, model.SMTPSpec{}, nil))
	code, lines, err := c.Cmd("EHLO client.example")
	if err != nil {
		t.Fatal(err)
	}
	if code != 250 {
		t.Fatalf("code=%d %v", code, lines)
	}
	text := smtptest.ReplyText(lines)
	for _, want := range []string{"SIZE 10485760", "8BITMIME", "SMTPUTF8", "ENHANCEDSTATUSCODES"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Contains(text, "STARTTLS") || strings.Contains(text, "AUTH") || strings.Contains(text, "PIPELINING") {
		t.Fatalf("unexpected extension in %q", text)
	}
}

func TestHideExtensions(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.HideExtensions = []string{"SMTPUTF8", "SIZE"}
	c := dial(t, startServer(t, spec, nil))
	_, lines, err := c.Cmd("EHLO x")
	if err != nil {
		t.Fatal(err)
	}
	text := smtptest.ReplyText(lines)
	if strings.Contains(text, "SMTPUTF8") || strings.Contains(text, "SIZE ") {
		t.Fatalf("hidden still present: %q", text)
	}
	if !strings.Contains(text, "8BITMIME") {
		t.Fatalf("8BITMIME missing: %q", text)
	}
}

func TestEmptyReversePath(t *testing.T) {
	c := dial(t, startServer(t, model.SMTPSpec{}, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<>")
	mustCmd(t, c, 250, "RCPT TO:<sink@lab.test>")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("Subject: bounce"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine(""); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, lines, err := c.ReadReply()
	if err != nil || code != 250 {
		t.Fatalf("code=%d %v %v", code, lines, err)
	}
	if !strings.Contains(strings.Join(lines, " "), "Queued as") {
		t.Fatalf("%v", lines)
	}
}

func TestDotUnstuff(t *testing.T) {
	sink := &stubSink{epoch: 1}
	c := dial(t, startServer(t, model.SMTPSpec{}, sink))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("..hidden"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, _, err := c.ReadReply()
	if err != nil || code != 250 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if sink.last == nil {
		t.Fatal("no insert")
	}
	raw := string(sink.last.Raw)
	if !strings.Contains(raw, ".hidden") || strings.Contains(raw, "..hidden") {
		t.Fatalf("raw=%q", raw)
	}
}

func TestLineTooLongCloses(t *testing.T) {
	c := dial(t, startServer(t, model.SMTPSpec{}, nil))
	long := "NOOP " + strings.Repeat("x", 600)
	code, _, err := c.Cmd(long)
	if err == nil && code != 500 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestMissingHeloDomain(t *testing.T) {
	c := dial(t, startServer(t, model.SMTPSpec{}, nil))
	mustCmd(t, c, 501, "EHLO")
	mustCmd(t, c, 501, "HELO")
}

func TestMailParamsAccepted(t *testing.T) {
	c := dial(t, startServer(t, model.SMTPSpec{}, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b> SIZE=12 BODY=8BITMIME SMTPUTF8")
}

func TestBadMailSyntax(t *testing.T) {
	c := dial(t, startServer(t, model.SMTPSpec{}, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 501, "MAIL FROM:")
	mustCmd(t, c, 501, "MAIL FROM:<a@b> SIZE=nope")
}

func TestParseHelpers(t *testing.T) {
	path, params, err := parsePathArg("FROM:<>", "FROM:")
	if err != nil || path != "" || params != "" {
		t.Fatalf("%q %q %v", path, params, err)
	}
	path, params, err = parsePathArg("TO:<a@b> NOTIFY=NEVER", "TO:")
	if err != nil || path != "a@b" || !strings.Contains(params, "NOTIFY") {
		t.Fatalf("%q %q %v", path, params, err)
	}
	n, set, err := parseMailParams("SIZE=9 BODY=8BITMIME")
	if err != nil || !set || n != 9 {
		t.Fatalf("%d %v %v", n, set, err)
	}
	if reserveBytes(10, true, 100) != 10 {
		t.Fatal("reserve declared")
	}
	if reserveBytes(0, false, 100) != 100 {
		t.Fatal("reserve max")
	}
}
