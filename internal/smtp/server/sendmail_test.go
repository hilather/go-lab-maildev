package server

import (
	"net/smtp"
	"testing"
)

func TestSendMailInterop(t *testing.T) {
	srv := startServer(t, defaultSMTPSpec(t), nil)
	msg := []byte("From: alice@lab.test\r\nTo: bob@lab.test\r\nSubject: interop\r\n\r\nhello from net/smtp\r\n")
	err := smtp.SendMail(srv.Addr().String(), nil, "alice@lab.test", []string{"bob@lab.test"}, msg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendMailEmptyFrom(t *testing.T) {
	srv := startServer(t, defaultSMTPSpec(t), nil)
	msg := []byte("Subject: bounce\r\n\r\n\r\n")
	err := smtp.SendMail(srv.Addr().String(), nil, "", []string{"sink@lab.test"}, msg)
	if err != nil {
		t.Fatal(err)
	}
}
