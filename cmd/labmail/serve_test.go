package main

import (
	"bytes"
	"context"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestServeInvalidDoesNotBind(t *testing.T) {
	path := testdataConfig(t, "invalid", "implicit-tls.yaml")
	var stdout, stderr bytes.Buffer
	code := serveCmd(context.Background(), []string{"--config", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d want 1 stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "smtp listen=") {
		t.Fatalf("invalid bootstrap bound SMTP: %q", stdout.String())
	}
}

func TestServeSendMail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := testdataConfig(t, "valid", "defaults.yaml")
	pid := filepath.Join(t.TempDir(), "labmail.pid")
	var stdout, stderr safeBuffer
	done := make(chan int, 1)
	go func() {
		done <- serveCmd(ctx, []string{
			"--config", path,
			"--smtp-listen", "127.0.0.1:0",
			"--pid-file", pid,
		}, &stdout, &stderr)
	}()

	addr := waitSMTPListen(t, &stdout)
	body, err := os.ReadFile(pid)
	if err != nil {
		t.Fatalf("pid file: %v stderr=%q", err, stderr.String())
	}
	if strings.TrimSpace(string(body)) == "" {
		t.Fatal("empty pid file")
	}

	msg := []byte("Subject: serve-interop\r\n\r\nvia labmail serve\r\n")
	if err := smtp.SendMail(addr, nil, "alice@lab.test", []string{"bob@lab.test"}, msg); err != nil {
		t.Fatalf("SendMail: %v stderr=%q", err, stderr.String())
	}

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("serve exit %d stderr=%q", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not exit")
	}
	if _, err := os.Stat(pid); !os.IsNotExist(err) {
		t.Fatalf("pid file remained: %v", err)
	}
}

func waitSMTPListen(t *testing.T, stdout *safeBuffer) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(stdout.String(), "\n") {
			if strings.HasPrefix(line, "labmail smtp listen=") {
				return strings.TrimPrefix(line, "labmail smtp listen=")
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("smtp listen not printed: %q", stdout.String())
	return ""
}
