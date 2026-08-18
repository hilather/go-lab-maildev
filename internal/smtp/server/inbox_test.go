package server

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtptest"
	"github.com/hilather/go-lab-maildev/internal/store"
)

func TestMemoryRejectMaps452(t *testing.T) {
	inbox, err := store.New(store.Options{MaxMessages: 1, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(inbox.Wipe)
	c := dial(t, startServer(t, defaultSMTPSpec(t), inbox))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	deliverDATA(t, c, 250, "one")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	deliverDATA(t, c, 452, "two")
}

func TestMemoryWipeDuringDATAMaps451(t *testing.T) {
	inbox, err := store.New(store.Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(inbox.Wipe)
	c := dial(t, startServer(t, defaultSMTPSpec(t), inbox))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	mustCmd(t, c, 354, "DATA")
	inbox.Wipe()
	if err := c.WriteLine("Subject: stale"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, _, err := c.ReadReply()
	if err != nil || code != 451 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if inbox.Stats().MessageCount != 0 {
		t.Fatal("stale insert stored")
	}
}

func TestMemoryStoresParsedMIME(t *testing.T) {
	inbox, err := store.New(store.Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: model.FullPolicyReject})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(inbox.Wipe)
	c := dial(t, startServer(t, defaultSMTPSpec(t), inbox))
	mustCmd(t, c, 250, "EHLO client.example")
	mustCmd(t, c, 250, "MAIL FROM:<alice@lab.test>")
	mustCmd(t, c, 250, "RCPT TO:<bob@lab.test>")
	mustCmd(t, c, 354, "DATA")
	for _, line := range []string{
		"From: Alice <alice@lab.test>",
		"To: bob@lab.test",
		"Subject: mime store",
		"Message-ID: <smtp@lab.test>",
		"",
		"hello inbox",
		".",
	} {
		if err := c.WriteLine(line); err != nil {
			t.Fatal(err)
		}
	}
	code, lines, err := c.ReadReply()
	if err != nil || code != 250 {
		t.Fatalf("code=%d lines=%v err=%v", code, lines, err)
	}
	id := queuedID(t, lines)
	if _, err := ulid.Parse(id); err != nil {
		t.Fatalf("queued id %q: %v", id, err)
	}
	got, err := inbox.Get(id, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "mime store" {
		t.Fatalf("subject=%q", got.Subject)
	}
	if !strings.Contains(got.Text, "hello inbox") {
		t.Fatalf("text=%q", got.Text)
	}
	if got.MessageID != "smtp@lab.test" {
		t.Fatalf("messageId=%q", got.MessageID)
	}
	if got.Envelope.From != "alice@lab.test" || got.Envelope.HELO != "client.example" {
		t.Fatalf("envelope=%+v", got.Envelope)
	}
}

func TestInsertTooLargeMaps552(t *testing.T) {
	sink := &stubSink{epoch: 1, err: store.ErrTooLarge}
	c := dial(t, startServer(t, defaultSMTPSpec(t), sink))
	mustCmd(t, c, 250, "EHLO x")
	mustCmd(t, c, 250, "MAIL FROM:<a@b>")
	mustCmd(t, c, 250, "RCPT TO:<c@d>")
	deliverDATA(t, c, 552, "too-big")
}

func deliverDATA(t *testing.T, c *smtptest.Client, want int, subject string) {
	t.Helper()
	mustCmd(t, c, 354, "DATA")
	if err := c.WriteLine("Subject: " + subject); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine(""); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("body"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteLine("."); err != nil {
		t.Fatal(err)
	}
	code, lines, err := c.ReadReply()
	if err != nil || code != want {
		t.Fatalf("DATA %s: code=%d want %d lines=%v err=%v", subject, code, want, lines, err)
	}
}

func queuedID(t *testing.T, lines []string) string {
	t.Helper()
	joined := strings.Join(lines, " ")
	const p = "Queued as "
	i := strings.LastIndex(joined, p)
	if i < 0 {
		t.Fatalf("no queued id in %q", joined)
	}
	return strings.TrimSpace(joined[i+len(p):])
}
