package server

import (
	"encoding/base64"
	"net/smtp"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/smtptest"
)

func TestAuthLoginTranscript(t *testing.T) {
	srv := startServer(t, authSpec(t, "lab", "secret"), nil)
	smtptest.PlayTranscript(t, srv.Addr().String(), testdataSMTP(t, "auth-login.txt"))
}

func TestAuthPlainTranscript(t *testing.T) {
	srv := startServer(t, authSpec(t, "lab", "secret"), nil)
	smtptest.PlayTranscript(t, srv.Addr().String(), testdataSMTP(t, "auth-plain.txt"))
}

func TestAuthPLAINTwoStep(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 334, "AUTH PLAIN")
	mustCmd(t, c, 235, "AGxhYgBzZWNyZXQ=")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
}

func TestAuthLOGINInitialResponse(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 334, "AUTH LOGIN "+base64.StdEncoding.EncodeToString([]byte("lab")))
	mustCmd(t, c, 235, base64.StdEncoding.EncodeToString([]byte("secret")))
}

func TestAuthCancel(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 334, "AUTH LOGIN")
	mustCmd(t, c, 501, "*")
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
}

func TestAuthFailed(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	code, lines, err := c.Cmd("AUTH PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00lab\x00nope")))
	if err != nil || code != 535 {
		t.Fatalf("code=%d %v %v", code, lines, err)
	}
	if !strings.Contains(strings.Join(lines, " "), "5.7.8") {
		t.Fatalf("%v", lines)
	}
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
}

func TestAuthEmptyCredentials(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 535, "AUTH LOGIN =")
}

func TestAuthRequiredBeforeMAIL(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
}

func TestAuthUnknownMechanism(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 504, "AUTH CRAM-MD5")
}

func TestAuthNoneNotAdvertised(t *testing.T) {
	c := dial(t, startServer(t, defaultSMTPSpec(t), nil))
	_, lines, err := c.Cmd("EHLO x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(smtptest.ReplyText(lines), "AUTH") {
		t.Fatalf("AUTH advertised: %v", lines)
	}
	mustCmd(t, c, 502, "AUTH PLAIN")
}

func TestHideAUTHExtension(t *testing.T) {
	spec := authSpec(t, "lab", "secret")
	spec.HideExtensions = []string{"AUTH"}
	c := dial(t, startServer(t, spec, nil))
	_, lines, err := c.Cmd("EHLO x")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(smtptest.ReplyText(lines), "AUTH") {
		t.Fatalf("hidden AUTH still present: %v", lines)
	}
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
}

func TestSwapSpecAuthRequiresMAIL(t *testing.T) {
	spec := defaultSMTPSpec(t)
	srv := startServer(t, spec, nil)
	c := dial(t, srv)
	mustCmd(t, c, 250, "EHLO x")
	next := authSpec(t, "lab", "secret")
	if err := srv.SwapSpec(next); err != nil {
		t.Fatal(err)
	}
	mustCmd(t, c, 530, "MAIL FROM:<a@b>")
	mustCmd(t, c, 235, "AUTH PLAIN AGxhYgBzZWNyZXQ=")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
}

func TestNewRejectsUnknownAuthMode(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.Auth.Mode = "cram"
	if _, err := New(Options{Address: "127.0.0.1:0", Spec: spec}); err == nil {
		t.Fatal("unknown auth mode should fail")
	}
}

func TestSendMailPlainAuth(t *testing.T) {
	srv := startServer(t, authSpec(t, "lab", "secret"), nil)
	auth := smtp.PlainAuth("", "lab", "secret", "127.0.0.1")
	msg := []byte("Subject: auth\r\n\r\nbody\r\n")
	if err := smtp.SendMail(srv.Addr().String(), auth, "alice@lab.test", []string{"bob@lab.test"}, msg); err != nil {
		t.Fatal(err)
	}
}

func TestSendMailAuthRequired(t *testing.T) {
	srv := startServer(t, authSpec(t, "lab", "secret"), nil)
	err := smtp.SendMail(srv.Addr().String(), nil, "alice@lab.test", []string{"bob@lab.test"}, []byte("Subject: x\r\n\r\n\r\n"))
	if err == nil {
		t.Fatal("expected 530 without AUTH")
	}
}

func TestNewAcceptsPlainLogin(t *testing.T) {
	if _, err := New(Options{Address: "127.0.0.1:0", Spec: authSpec(t, "lab", "secret")}); err != nil {
		t.Fatal(err)
	}
}

func TestAlreadyAuthenticated(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 235, "AUTH PLAIN AGxhYgBzZWNyZXQ=")
	mustCmd(t, c, 503, "AUTH LOGIN")
}

func TestAuthBeforeHello(t *testing.T) {
	c := dial(t, startServer(t, authSpec(t, "lab", "secret"), nil))
	mustCmd(t, c, 503, "AUTH PLAIN AGxhYgBzZWNyZXQ=")
}
