package server

import (
	"bytes"
	"context"
	"net/smtp"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/observability"
	"github.com/hilather/go-lab-maildev/internal/store"
)

func TestSMTPMetricsOnSendMail(t *testing.T) {
	reg := observability.NewRegistry()
	var logs bytes.Buffer
	log := observability.NewLogger(&logs, observability.LevelInfo)
	inbox, err := store.New(store.Options{MaxMessages: 10, MaxBytes: 1 << 20, FullPolicy: "reject"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(inbox.Wipe)
	inbox.SetTelemetry(reg, log)
	spec := defaultSMTPSpec(t)
	srv, err := New(Options{Address: "127.0.0.1:0", Spec: spec, Store: inbox, Metrics: reg, Logger: log})
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

	msg := []byte("Subject: obs\r\n\r\nhello\r\n")
	if err := smtp.SendMail(srv.Addr().String(), nil, "a@lab.test", []string{"b@lab.test"}, msg); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if v, ok := reg.Get(observability.MetricSMTPMessagesTotal, map[string]string{"result": "accepted"}); !ok || v < 1 {
		t.Fatalf("messages accepted=%v ok=%v", v, ok)
	}
	if v, ok := reg.Get(observability.MetricSMTPSessionsTotal, map[string]string{"result": "ok"}); !ok || v < 1 {
		t.Fatalf("sessions ok=%v ok=%v", v, ok)
	}
	if v, ok := reg.Get(observability.MetricStoreMessages, nil); !ok || v != 1 {
		t.Fatalf("store messages=%v ok=%v", v, ok)
	}
	out := logs.String()
	if !bytes.Contains([]byte(out), []byte(observability.EventSMTPAccepted)) {
		t.Fatalf("missing smtp.accepted: %s", out)
	}
	if bytes.Contains([]byte(out), []byte("Subject: obs")) {
		t.Fatal("logged raw subject/DATA")
	}
}

func TestAcceptingFalseAfterShutdown(t *testing.T) {
	srv := startServer(t, defaultSMTPSpec(t), nil)
	if !srv.Accepting() || srv.Addr() == nil {
		t.Fatal("expected accepting after Start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if srv.Accepting() {
		t.Fatal("Accepting must be false after Shutdown")
	}
	if srv.Addr() == nil {
		t.Fatal("Addr stays set after Shutdown so callers can log the former bind")
	}
}
