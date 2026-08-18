package main

import (
	"bytes"
	"context"
	"net/http"
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

func TestServeAcceptsAuthMode(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "smtp.pass")
	if err := os.WriteFile(pw, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  smtp:\n    auth:\n      mode: plain_login\n      username: lab\n      passwordFile: " + pw + "\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr safeBuffer
	done := make(chan int, 1)
	go func() {
		done <- serveCmd(ctx, []string{"--config", cfg, "--smtp-listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()
	addr := waitSMTPListen(t, &stdout)
	auth := smtp.PlainAuth("", "lab", "secret", "127.0.0.1")
	msg := []byte("Subject: serve-auth\r\n\r\nvia labmail serve AUTH\r\n")
	if err := smtp.SendMail(addr, auth, "alice@lab.test", []string{"bob@lab.test"}, msg); err != nil {
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
			"--management-listen", "off",
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

func TestServeBindsManagement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := testdataConfig(t, "valid", "defaults.yaml")
	var stdout, stderr safeBuffer
	done := make(chan int, 1)
	go func() {
		done <- serveCmd(ctx, []string{
			"--config", path,
			"--smtp-listen", "127.0.0.1:0",
			"--management-listen", "127.0.0.1:0",
		}, &stdout, &stderr)
	}()
	_ = waitSMTPListen(t, &stdout)
	mgmt := waitPrefix(t, &stdout, "labmail management listen=")
	var resp *http.Response
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://" + mgmt + "/v1/health/ready")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("ready: %v stderr=%q", err, stderr.String())
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("ready status=%d", resp.StatusCode)
	}

	hz, err := http.Get("http://" + mgmt + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	if hz.StatusCode != 200 {
		_ = hz.Body.Close()
		t.Fatalf("healthz status=%d", hz.StatusCode)
	}
	_ = hz.Body.Close()
	email, err := http.Get("http://" + mgmt + "/email")
	if err != nil {
		t.Fatalf("email: %v", err)
	}
	if email.StatusCode != 200 {
		_ = email.Body.Close()
		t.Fatalf("email status=%d", email.StatusCode)
	}
	_ = email.Body.Close()
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("serve exit %d stderr=%q", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not exit")
	}
}

func TestServeCompatDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := testdataConfig(t, "valid", "ui-disabled.yaml")
	var stdout, stderr safeBuffer
	done := make(chan int, 1)
	go func() {
		done <- serveCmd(ctx, []string{
			"--config", path,
			"--smtp-listen", "127.0.0.1:0",
			"--management-listen", "127.0.0.1:0",
		}, &stdout, &stderr)
	}()
	_ = waitSMTPListen(t, &stdout)
	mgmt := waitPrefix(t, &stdout, "labmail management listen=")
	var resp *http.Response
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://" + mgmt + "/v1/health/ready")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("ready: %v stderr=%q", err, stderr.String())
	}
	_ = resp.Body.Close()
	email, err := http.Get("http://" + mgmt + "/email")
	if err != nil {
		t.Fatalf("email: %v", err)
	}
	if email.StatusCode != http.StatusNotFound {
		_ = email.Body.Close()
		t.Fatalf("compatEnabled false: GET /email status=%d", email.StatusCode)
	}
	_ = email.Body.Close()
	for _, path := range []string{"/healthz", "/config"} {
		resp, err := http.Get("http://" + mgmt + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if resp.StatusCode != http.StatusNotFound {
			_ = resp.Body.Close()
			t.Fatalf("compatEnabled false: GET %s status=%d", path, resp.StatusCode)
		}
		_ = resp.Body.Close()
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
}

func waitSMTPListen(t *testing.T, stdout *safeBuffer) string {
	t.Helper()
	return waitPrefix(t, stdout, "labmail smtp listen=")
}

func waitPrefix(t *testing.T, stdout *safeBuffer, prefix string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(stdout.String(), "\n") {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimPrefix(line, prefix)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s not printed: %q", prefix, stdout.String())
	return ""
}
