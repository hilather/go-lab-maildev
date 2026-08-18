package server

import (
	"context"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/store"
)

type stubSink struct {
	err   error
	epoch uint64
	last  *model.Message
}

func (s *stubSink) Insert(_ context.Context, epoch uint64, msg *model.Message) (model.InsertResult, error) {
	if s.epoch != 0 && epoch != s.epoch {
		return model.InsertResult{}, store.ErrStaleEpoch
	}
	s.last = msg
	if s.err != nil {
		return model.InsertResult{}, s.err
	}
	return model.InsertResult{ID: "stub-1"}, nil
}

func (s *stubSink) Epoch() uint64 { return s.epoch }

func TestInsertFullMaps452(t *testing.T) {
	sink := &stubSink{epoch: 1, err: store.ErrFull}
	c := dial(t, startServer(t, defaultSMTPSpec(t), sink))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, _, err := c.ReadReply()
	if err != nil || code != 452 {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestInsertRecordsEnvelope(t *testing.T) {
	sink := &stubSink{epoch: 1}
	c := dial(t, startServer(t, defaultSMTPSpec(t), sink))
	mustCmd(t, c, 250, "EHLO client.example")
	mustCmd(t, c, 250, "MAIL FROM:<alice@lab.test>")
	mustCmd(t, c, 250, "RCPT TO:<bob@lab.test>")
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("Subject: rec"); err != nil {
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
	if sink.last.Envelope.From != "alice@lab.test" || sink.last.Envelope.HELO != "client.example" {
		t.Fatalf("%+v", sink.last.Envelope)
	}
	if len(sink.last.Envelope.To) != 1 || sink.last.Envelope.To[0] != "bob@lab.test" {
		t.Fatalf("to=%v", sink.last.Envelope.To)
	}
	if !strings.Contains(string(sink.last.Raw), "Subject: rec") {
		t.Fatalf("raw=%q", sink.last.Raw)
	}
}

func TestNewRequiresAddress(t *testing.T) {
	_, err := New(Options{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewRejectsAuthAndTLS(t *testing.T) {
	spec := defaultSMTPSpec(t)
	spec.TLS.Mode = model.TLSModeImplicit
	if _, err := New(Options{Address: "127.0.0.1:0", Spec: spec}); err == nil {
		t.Fatal("implicit should fail closed")
	}
	spec = defaultSMTPSpec(t)
	spec.TLS.Required = true
	if _, err := New(Options{Address: "127.0.0.1:0", Spec: spec}); err == nil {
		t.Fatal("tls.required without starttls should fail closed")
	}
}
