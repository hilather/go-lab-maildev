package server

import (
	"crypto/tls"
	"io"
	"net/smtp"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtptest"
)

func TestSTARTTLSOptionalTranscript(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.TLS.Mode = model.TLSModeStartTLS
	srv := startServer(t, spec, nil)
	smtptest.PlayTranscript(t, srv.Addr().String(), testdataSMTP(t, "starttls-optional.txt"))
}

func TestSTARTTLSRequiredTranscript(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.TLS.Mode = model.TLSModeStartTLS
	spec.TLS.Required = true
	srv := startServer(t, spec, nil)
	smtptest.PlayTranscript(t, srv.Addr().String(), testdataSMTP(t, "starttls-required.txt"))
}

func TestSTARTTLSHandshake(t *testing.T) {
	srv := startServer(t, starttlsSpec(t, false), nil)
	c := dial(t, srv)
	_, lines, err := c.Cmd("EHLO x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(smtptest.ReplyText(lines), "STARTTLS") {
		t.Fatalf("STARTTLS missing: %v", lines)
	}
	mustCmd(t, c, 220, "STARTTLS")
	if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatal(err)
	}
	_, lines, err = c.Cmd("EHLO x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(smtptest.ReplyText(lines), "STARTTLS") {
		t.Fatalf("STARTTLS still advertised after handshake: %v", lines)
	}
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("Subject: tls"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, _, err := c.ReadReply()
	if err != nil || code != 250 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestSTARTTLSRequiredBlocksCleartext(t *testing.T) {
	c := dial(t, startServer(t, starttlsSpec(t, true), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
	mustCmd(t, c, 220, "STARTTLS")
	if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatal(err)
	}
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
}

func TestTLSOffNotAdvertised(t *testing.T) {
	c := dial(t, startServer(t, defaultSMTPSpec(t), nil))
	_, lines, err := c.Cmd("EHLO x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(smtptest.ReplyText(lines), "STARTTLS") {
		t.Fatalf("STARTTLS advertised: %v", lines)
	}
	mustCmd(t, c, 502, "STARTTLS")
}

func TestHideSTARTTLSExtension(t *testing.T) {
	spec := starttlsSpec(t, true)
	spec.HideExtensions = []string{"STARTTLS"}
	c := dial(t, startServer(t, spec, nil))
	_, lines, err := c.Cmd("EHLO x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(smtptest.ReplyText(lines), "STARTTLS") {
		t.Fatalf("hidden STARTTLS still present: %v", lines)
	}
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
}

func TestSwapSpecTLSRequired(t *testing.T) {
	spec := defaultSMTPSpec(t)
	srv := startServer(t, spec, nil)
	c := dial(t, srv)
	mustCmd(t, c, 250, "EHLO x")
	next := starttlsSpec(t, true)
	if err := srv.SwapSpec(next); err != nil {
		t.Fatal(err)
	}
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
}

func TestNewRejectsImplicitTLS(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.TLS.Mode = model.TLSModeImplicit
	if _, err := New(Options{Address: "127.0.0.1:0", Spec: spec}); err == nil {
		t.Fatal("implicit must stay rejected")
	}
	if err := startServer(t, defaultSMTPSpec(t), nil).SwapSpec(spec); err == nil {
		t.Fatal("SwapSpec must reject implicit")
	}
}

func TestNewRejectsRequiredWithoutStartTLS(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.TLS.Required = true
	if _, err := New(Options{Address: "127.0.0.1:0", Spec: spec}); err == nil {
		t.Fatal("required without starttls should fail")
	}
}

func TestNewAcceptsStartTLS(t *testing.T) {
	if _, err := New(Options{Address: "127.0.0.1:0", Spec: starttlsSpec(t, true)}); err != nil {
		t.Fatal(err)
	}
}

func TestSTARTTLSMissingCert(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.TLS.Mode = model.TLSModeStartTLS
	c := dial(t, startServer(t, spec, nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 454, "STARTTLS")
}

func TestNetSMTPStartTLS(t *testing.T) {
	srv := startServer(t, starttlsSpec(t, true), nil)
	c, err := smtp.Dial(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if err := c.Hello("client.example"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := c.Extension("STARTTLS"); !ok {
		t.Fatal("STARTTLS not advertised")
	}
	if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatal(err)
	}
	if err := c.Mail("alice@lab.test"); err != nil {
		t.Fatal(err)
	}
	if err := c.Rcpt("bob@lab.test"); err != nil {
		t.Fatal(err)
	}
	w, err := c.Data()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "Subject: net-smtp-tls\r\n\r\nbody\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
