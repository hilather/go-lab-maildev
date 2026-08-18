package perf

import (
	"context"
	"flag"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/model"
	smtpserver "github.com/hilather/go-lab-maildev/internal/smtp/server"
	"github.com/hilather/go-lab-maildev/internal/store"
)

// defaultSoakN is small so CI stays short. This is completeness under
// accept/wait/wipe, not a QPS gate.
const defaultSoakN = 8

var soakNFlag = flag.Int("soak-n", defaultSoakN, "messages to accept during soak (CI default 8)")

func soakN(t *testing.T) int {
	t.Helper()
	n := *soakNFlag
	if env := os.Getenv("LABMAIL_SOAK_N"); env != "" {
		parsed, err := strconv.Atoi(env)
		if err != nil {
			t.Fatalf("LABMAIL_SOAK_N: %v", err)
		}
		n = parsed
	}
	if testing.Short() && n > 2 {
		n = 2
	}
	if n < 1 {
		n = 1
	}
	return n
}

func TestSoakAcceptWaitWipe(t *testing.T) {
	n := soakN(t)
	inbox := newInbox(t, n)
	srv := startSMTP(t, inbox)
	addr := srv.Addr().String()

	for i := 0; i < n; i++ {
		subject := fmt.Sprintf("soak-%d", i)
		msg := []byte("From: soak@lab.test\r\nTo: inbox@lab.test\r\nSubject: " + subject + "\r\n\r\nbody " + subject + "\r\n")
		if err := smtp.SendMail(addr, nil, "soak@lab.test", []string{"inbox@lab.test"}, msg); err != nil {
			t.Fatalf("accept %d: %v", i, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := inbox.Wait(ctx, model.MessageFilter{Subject: fmt.Sprintf("soak-%d", n-1)})
	if err != nil {
		t.Fatalf("wait last: %v", err)
	}
	if got == nil || got.Subject != fmt.Sprintf("soak-%d", n-1) {
		t.Fatalf("wait last = %+v", got)
	}

	st := inbox.Stats()
	if st.MessageCount != n {
		t.Fatalf("message count %d want %d", st.MessageCount, n)
	}
	epoch := inbox.Epoch()
	gen := inbox.Generation()

	inbox.Wipe()
	after := inbox.Stats()
	if after.MessageCount != 0 || after.Bytes != 0 {
		t.Fatalf("wipe left occupancy count=%d bytes=%d", after.MessageCount, after.Bytes)
	}
	if inbox.Epoch() == epoch {
		t.Fatal("wipe did not bump epoch")
	}
	if inbox.Generation() == gen {
		t.Fatal("wipe did not bump storeGeneration")
	}

	// Wait after wipe must not resurrect deleted mail.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer waitCancel()
	resurrected, err := inbox.Wait(waitCtx, model.MessageFilter{})
	if resurrected != nil {
		t.Fatalf("wait after wipe returned %s", resurrected.ID)
	}
	if err == nil {
		t.Fatal("wait after wipe succeeded")
	}
}

func TestSoakWaitThenAcceptThenWipe(t *testing.T) {
	n := soakN(t)
	inbox := newInbox(t, n)
	srv := startSMTP(t, inbox)
	addr := srv.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		got, err := inbox.Wait(ctx, model.MessageFilter{SubjectContains: "soak"})
		if err != nil {
			errc <- err
			return
		}
		if got == nil {
			errc <- fmt.Errorf("wait returned nil")
			return
		}
		errc <- nil
	}()

	// Let the waiter park on the cond before the first insert.
	time.Sleep(20 * time.Millisecond)
	for i := 0; i < n; i++ {
		subject := fmt.Sprintf("soak-parked-%d", i)
		msg := []byte("From: soak@lab.test\r\nTo: inbox@lab.test\r\nSubject: " + subject + "\r\n\r\nbody\r\n")
		if err := smtp.SendMail(addr, nil, "soak@lab.test", []string{"inbox@lab.test"}, msg); err != nil {
			t.Fatalf("accept %d: %v", i, err)
		}
	}

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("parked wait: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("parked wait did not return")
	}

	if inbox.Stats().MessageCount != n {
		t.Fatalf("message count %d want %d", inbox.Stats().MessageCount, n)
	}
	inbox.Wipe()
	if inbox.Stats().MessageCount != 0 {
		t.Fatal("wipe left messages")
	}
}

func newInbox(t *testing.T, n int) *store.Memory {
	t.Helper()
	inbox, err := store.New(store.Options{
		MaxMessages: n + 8,
		MaxBytes:    16 << 20,
		FullPolicy:  model.FullPolicyReject,
		MaxWait:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(inbox.Wipe)
	return inbox
}

func startSMTP(t *testing.T, sink store.Sink) *smtpserver.Server {
	t.Helper()
	st, err := config.Load([]byte("apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: soak\nspec: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := smtpserver.New(smtpserver.Options{
		Address: "127.0.0.1:0",
		Spec:    st.Spec.SMTP,
		Store:   sink,
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
	return srv
}
