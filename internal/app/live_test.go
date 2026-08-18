package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtp/server"
	"github.com/hilather/go-lab-maildev/internal/smtptest"
)

func TestApplyHideExtensionsLiveOnEHLO(t *testing.T) {
	svc, boot := mustBoot(t)
	srv, err := server.New(server.Options{
		Address:   "127.0.0.1:0",
		Spec:      boot.Canonical.Spec.SMTP,
		Store:     svc.Inbox(),
		Snapshots: svc.Snapshots(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	c, err := smtptest.Dial(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if code, lines, err := c.Cmd("EHLO first"); err != nil || code != 250 {
		t.Fatalf("ehlo1 %d %v", code, err)
	} else {
		sawSIZE := false
		for _, ln := range lines {
			if strings.Contains(ln, "SIZE ") {
				sawSIZE = true
			}
		}
		if !sawSIZE {
			t.Fatalf("initial EHLO missing SIZE: %v", lines)
		}
	}
	if _, err := svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Reason:           "hide SIZE",
		Operations:       []model.Operation{hideSIZE()},
	}); err != nil {
		t.Fatal(err)
	}
	if code, lines, err := c.Cmd("EHLO second"); err != nil || code != 250 {
		t.Fatalf("ehlo2 %d %v", code, err)
	} else {
		for _, ln := range lines {
			if strings.Contains(ln, "SIZE ") {
				t.Fatalf("SIZE still advertised after apply: %v", lines)
			}
		}
	}
}

func TestApplyMaxRecipientsLiveOnRCPT(t *testing.T) {
	svc, boot := mustBoot(t)
	// Compile path cannot change maxRecipients via a 1.0 op, so install a
	// candidate snapshot the same way Apply does: copy, tweak, compile is
	// not exposed. Exercise live re-read by swapping a compiled-looking
	// snapshot after hide-extensions apply (which goes through Service)
	// plus a direct snapshot swap of MaxRecipients — that is the same
	// pointer SMTP reads.
	srv, err := server.New(server.Options{
		Address:   "127.0.0.1:0",
		Spec:      boot.Canonical.Spec.SMTP,
		Store:     svc.Inbox(),
		Snapshots: svc.Snapshots(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	_, err = svc.Apply(context.Background(), actor(), ChangeIn{
		ExpectedRevision: boot.Revision,
		Reason:           "hide SMTPUTF8",
		Operations: []model.Operation{{
			Op:             model.OpReplaceHideExtensions,
			HideExtensions: []string{"SMTPUTF8"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	live := svc.Active()
	cp := *live
	st := *live.Canonical
	st.Spec.SMTP.MaxRecipients = 1
	cp.Canonical = &st
	svc.Snapshots().Swap(&cp)

	c, err := smtptest.Dial(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if code, _, err := c.Cmd("EHLO x"); err != nil || code != 250 {
		t.Fatalf("ehlo %d %v", code, err)
	}
	if code, _, err := c.Cmd("MAIL FROM:<a@b>"); err != nil || code != 250 {
		t.Fatalf("mail %d %v", code, err)
	}
	if code, _, err := c.Cmd("RCPT TO:<one@b>"); err != nil || code != 250 {
		t.Fatalf("rcpt1 %d %v", code, err)
	}
	if code, _, err := c.Cmd("RCPT TO:<two@b>"); err != nil || code != 452 {
		t.Fatalf("rcpt2 %d %v (want 452 after live maxRecipients=1)", code, err)
	}
}

func TestResetDuringDATAMaps451(t *testing.T) {
	svc, boot := mustBoot(t)
	srv, err := server.New(server.Options{
		Address:   "127.0.0.1:0",
		Spec:      boot.Canonical.Spec.SMTP,
		Store:     svc.Inbox(),
		Snapshots: svc.Snapshots(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	c, err := smtptest.Dial(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	for _, line := range []string{"EHLO x", "MAIL FROM:<a@b>", "RCPT TO:<c@d>"} {
		if code, _, err := c.Cmd(line); err != nil || code != 250 {
			t.Fatalf("%s -> %d %v", line, code, err)
		}
	}
	if code, _, err := c.Cmd("DATA"); err != nil || code != 354 {
		t.Fatalf("data %d %v", code, err)
	}
	if _, err := svc.Reset(context.Background(), actor(), ResetIn{Reason: "mid-data"}); err != nil {
		t.Fatal(err)
	}
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
	if svc.Inbox().Stats().MessageCount != 0 {
		t.Fatal("stale insert stored")
	}
}
