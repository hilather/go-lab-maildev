package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
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
		done <- serveCmd(ctx, []string{"--config", cfg, "--smtp-listen", "127.0.0.1:0", "--management-listen", "off"}, &stdout, &stderr)
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

const serveTestToken = "0123456789abcdef0123456789abcdef"

func writeServeConfig(t *testing.T, metricsListen string, publicPath bool) string {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(serveTestToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer_and_basic\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n  observability:\n    metrics:\n      listen: " + strconvQuote(metricsListen) + "\n      publicPath: " + strconv.FormatBool(publicPath) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func strconvQuote(s string) string {
	return `"` + s + `"`
}

func TestServeSendMail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := writeServeConfig(t, "", false)
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
	path := writeServeConfig(t, "", false)
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
			_ = resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("ready: %v stderr=%q", err, stderr.String())
	}
	defer func() { _ = resp.Body.Close() }()
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
	if email.StatusCode != http.StatusUnauthorized {
		_ = email.Body.Close()
		t.Fatalf("email status=%d want 401 (default YAML is bearer_and_basic)", email.StatusCode)
	}
	_ = email.Body.Close()
	mcpReq, reqErr := http.NewRequest(http.MethodGet, "http://"+mgmt+"/mcp", nil)
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	mcpResp, err := http.DefaultClient.Do(mcpReq)
	if err != nil {
		t.Fatalf("mcp GET: %v", err)
	}
	_ = mcpResp.Body.Close()
	if mcpResp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("mcp GET status=%d want 405", mcpResp.StatusCode)
	}
	unauth := doHTTP(t, "http://"+mgmt+"/v1/metrics")
	defer func() { _ = unauth.Body.Close() }()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status=%d want 401", unauth.StatusCode)
	}
	hidden := doHTTPAuth(t, "http://"+mgmt+"/v1/metrics", serveTestToken)
	defer func() { _ = hidden.Body.Close() }()
	if hidden.StatusCode != http.StatusNotFound {
		t.Fatalf("publicPath false: metrics status=%d", hidden.StatusCode)
	}

	var hcOut, hcErr bytes.Buffer
	if code := healthcheckCmd([]string{"--url", "http://" + mgmt + "/v1/health/ready"}, &hcOut, &hcErr); code != 0 {
		t.Fatalf("healthcheck exit %d stderr=%q", code, hcErr.String())
	}
	if !strings.Contains(hcOut.String(), "ok") {
		t.Fatalf("healthcheck stdout=%q", hcOut.String())
	}
	ui, err := http.Get("http://" + mgmt + "/")
	if err != nil {
		t.Fatalf("ui: %v", err)
	}
	raw, err := io.ReadAll(ui.Body)
	_ = ui.Body.Close()
	if err != nil {
		t.Fatalf("ui body: %v", err)
	}
	if ui.StatusCode != 200 || !strings.Contains(string(raw), "LabMail") {
		t.Fatalf("GET / status=%d body=%s", ui.StatusCode, raw)
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
			_ = resp.Body.Close()
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
	ui, err := http.Get("http://" + mgmt + "/")
	if err != nil {
		t.Fatalf("ui: %v", err)
	}
	if ui.StatusCode != http.StatusNotFound {
		_ = ui.Body.Close()
		t.Fatalf("ui.enabled false: GET / status=%d", ui.StatusCode)
	}
	_ = ui.Body.Close()
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

func TestServeMetricsListenAndPublicPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := writeServeConfig(t, "127.0.0.1:0", true)
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
	metricsAddr := waitPrefix(t, &stdout, "labmail metrics listen=")

	pub := waitHTTPAuth(t, "http://"+mgmt+"/v1/metrics", serveTestToken)
	defer func() { _ = pub.Body.Close() }()
	if pub.StatusCode != 200 {
		t.Fatalf("publicPath true: status=%d", pub.StatusCode)
	}
	if !strings.Contains(pub.Header.Get("Content-Type"), "openmetrics") {
		t.Fatalf("content-type=%s", pub.Header.Get("Content-Type"))
	}

	scrape := waitHTTP(t, "http://"+metricsAddr+"/metrics")
	defer func() { _ = scrape.Body.Close() }()
	if scrape.StatusCode != 200 {
		t.Fatalf("scrape status=%d", scrape.StatusCode)
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

func waitHTTP(t *testing.T, url string) *http.Response {
	t.Helper()
	return waitHTTPAuth(t, url, "")
}

func waitHTTPAuth(t *testing.T, url, token string) *http.Response {
	t.Helper()
	var resp *http.Response
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequest(http.MethodGet, url, nil)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			return resp
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("GET %s: %v", url, err)
	return nil
}

func doHTTP(t *testing.T, url string) *http.Response {
	t.Helper()
	return waitHTTP(t, url)
}

func doHTTPAuth(t *testing.T, url, token string) *http.Response {
	t.Helper()
	return waitHTTPAuth(t, url, token)
}

func TestReadyUnreadyAfterSMTPShutdown(t *testing.T) {
	ctx := context.Background()
	path := writeServeConfig(t, "", false)
	rt, err := serveFromConfig(ctx, serveFlags{
		Config:           path,
		SMTPListen:       "127.0.0.1:0",
		ManagementListen: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		shctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = rt.shutdown(shctx)
	})
	readyURL := "http://" + rt.http.Addr() + "/v1/health/ready"
	statusURL := "http://" + rt.http.Addr() + "/v1/status"
	got := waitHTTP(t, readyURL)
	if got.StatusCode != http.StatusOK {
		_ = got.Body.Close()
		t.Fatalf("ready before smtp stop: %d", got.StatusCode)
	}
	_ = got.Body.Close()
	shctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := rt.smtp.Shutdown(shctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	if rt.smtp.Accepting() {
		t.Fatal("SMTP still accepting after Shutdown")
	}

	deadline := time.Now().Add(2 * time.Second)
	var ready *http.Response
	for time.Now().Before(deadline) {
		ready, err = http.Get(readyURL)
		if err == nil && ready.StatusCode == http.StatusServiceUnavailable {
			break
		}
		if ready != nil {
			_ = ready.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ready.Body.Close() }()
	if ready.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("ready after smtp stop: %d", ready.StatusCode)
	}
	st := waitHTTPAuth(t, statusURL, serveTestToken)
	defer func() { _ = st.Body.Close() }()
	if st.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", st.StatusCode)
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
