package rest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
)

const testRotatorToken = "abcdef0123456789abcdef0123456789"

// TestEventsStreamStopsAfterBearerRevocation: a bearer stream ends, and does
// not deliver mail.received, after that token is removed and reset.
func TestEventsStreamStopsAfterBearerRevocation(t *testing.T) {
	dir := t.TempDir()
	adminPath := filepath.Join(dir, "admin.token")
	rotatorPath := filepath.Join(dir, "rotator.token")
	if err := os.WriteFile(adminPath, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rotatorPath, []byte(testRotatorToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := bearerPairYAML(adminPath, rotatorPath)
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, svc := bootConfiguredServer(t, cfgPath, 0)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	frames, _, done := openAuthedStream(t, ts, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
	})

	if err := os.WriteFile(cfgPath, []byte(bearerOnlyYAML("rotator", rotatorPath, "administrator")), 0o644); err != nil {
		t.Fatal(err)
	}
	resetReq, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/state:reset", strings.NewReader(`{"reason":"remove admin"}`))
	if err != nil {
		t.Fatal(err)
	}
	resetReq.Header.Set("Content-Type", "application/json")
	resetReq.Header.Set("Authorization", "Bearer "+testRotatorToken)
	resetRes, err := http.DefaultClient.Do(resetReq)
	if err != nil {
		t.Fatal(err)
	}
	resetBody, _ := io.ReadAll(resetRes.Body)
	_ = resetRes.Body.Close()
	if resetRes.StatusCode != http.StatusOK {
		t.Fatalf("reset status=%d body=%s", resetRes.StatusCode, resetBody)
	}
	if status := authedStatus(t, ts, "Bearer "+testBearerToken); status != http.StatusUnauthorized {
		t.Fatalf("removed bearer status=%d want 401", status)
	}

	insertMail(t, svc, "after-bearer-revoke", "body")
	assertStreamEndedWithoutMail(t, frames, done)
}

// TestEventsStreamStopsAfterBearerLosesMailRead: demoting the bearer to a
// role whose scopes omit mail.read ends the stream. Every built-in role
// includes mail.read, and mail.admin satisfies it, so the demotion sets an
// explicit scope list of mail.audit.read.
func TestEventsStreamStopsAfterBearerLosesMailRead(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := bearerOnlyYAML("admin", tok, "administrator")
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, svc := bootConfiguredServer(t, cfgPath, 0)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	frames, _, done := openAuthedStream(t, ts, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
	})

	demoted := strings.Replace(body, "role: administrator", "role: viewer\n          scopes: [mail.audit.read]", 1)
	if err := os.WriteFile(cfgPath, []byte(demoted), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "demote"}); err != nil {
		t.Fatal(err)
	}
	if status := authedStatus(t, ts, "Bearer "+testBearerToken); status != http.StatusForbidden {
		t.Fatalf("demoted bearer status=%d want 403", status)
	}

	insertMail(t, svc, "after-demote", "body")
	assertStreamEndedWithoutMail(t, frames, done)
}

// TestEventsStreamStopsAfterBasicPasswordChange: a Basic stream ends after
// the password file changes and reset reloads the verifier.
func TestEventsStreamStopsAfterBasicPasswordChange(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	pw := filepath.Join(dir, "pass")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pw, []byte("lab-web-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer_and_basic\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n      basic:\n        username: admin\n        passwordFile: " + pw + "\n        tokenRef: admin\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, svc := bootConfiguredServer(t, cfgPath, 0)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	frames, _, done := openAuthedStream(t, ts, func(req *http.Request) {
		req.SetBasicAuth("admin", "lab-web-pass")
	})

	if err := os.WriteFile(pw, []byte("new-web-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "rotate-basic"}); err != nil {
		t.Fatal(err)
	}
	old, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/messages", nil)
	if err != nil {
		t.Fatal(err)
	}
	old.SetBasicAuth("admin", "lab-web-pass")
	oldRes, err := http.DefaultClient.Do(old)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(oldRes.Body)
	_ = oldRes.Body.Close()
	if oldRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old basic password status=%d want 401", oldRes.StatusCode)
	}

	insertMail(t, svc, "after-basic-rotate", "body")
	assertStreamEndedWithoutMail(t, frames, done)
}

// TestEventsStreamLiveBearerKeepsDelivering: a bearer that is still valid
// passes the per-event and heartbeat recheck and still receives mail.received.
func TestEventsStreamLiveBearerKeepsDelivering(t *testing.T) {
	s, svc, _ := newAuthServer(t)
	s.sseHeartbeat = 25 * time.Millisecond
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	frames, heartbeats, done := openAuthedStream(t, ts, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
	})
	select {
	case <-heartbeats:
	case <-done:
		t.Fatal("valid bearer stream closed on heartbeat")
	case <-time.After(time.Second):
		t.Fatal("missing heartbeat")
	}
	insertMail(t, svc, "still-valid", "body")
	ev := waitSSE(t, frames, done, app.InboxMailReceived)
	if ev.data["subject"] != "still-valid" {
		t.Fatalf("received=%+v", ev)
	}
}

// TestEventsStreamStopsWhenLoopbackModeEnds: a dev-loopback-unauth stream
// ends once reset leaves that mode. A stale cookie is not what authorized it.
func TestEventsStreamStopsWhenLoopbackModeEnds(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: dev-loopback-unauth\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, svc := bootConfiguredServer(t, cfgPath, 0)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	frames, _, done := openAuthedStream(t, ts, func(req *http.Request) {})

	if err := os.WriteFile(cfgPath, []byte(bearerOnlyYAML("admin", tok, "administrator")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "leave-loopback"}); err != nil {
		t.Fatal(err)
	}
	insertMail(t, svc, "after-loopback", "body")
	assertStreamEndedWithoutMail(t, frames, done)
}

func bearerPairYAML(adminPath, rotatorPath string) string {
	return "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + adminPath + "\n          role: administrator\n        - id: rotator\n          secretFile: " + rotatorPath + "\n          role: administrator\n"
}

func bearerOnlyYAML(id, path, role string) string {
	return "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: " + id + "\n          secretFile: " + path + "\n          role: " + role + "\n"
}

func bootConfiguredServer(t *testing.T, cfgPath string, hb time.Duration) (*Server, *app.App) {
	t.Helper()
	svc, err := app.Boot(t.Context(), app.Options{BootstrapPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: v, RatePerSec: -1, SSEHeartbeat: hb})
	if err != nil {
		t.Fatal(err)
	}
	return s, svc
}

func openAuthedStream(t *testing.T, ts *httptest.Server, prep func(*http.Request)) (<-chan sseTestFrame, <-chan struct{}, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	prep(req)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("stream status=%d body=%s", res.StatusCode, b)
	}
	frames, heartbeats, done := readSSE(res.Body)
	return frames, heartbeats, done
}

func authedStatus(t *testing.T, ts *httptest.Server, authorization string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/messages", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", authorization)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res.StatusCode
}

func assertStreamEndedWithoutMail(t *testing.T, frames <-chan sseTestFrame, done <-chan struct{}) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case ev := <-frames:
			if ev.event == app.InboxMailReceived {
				t.Fatal("stream delivered mail.received after the authorizing credential was revoked")
			}
		case <-done:
			select {
			case ev := <-frames:
				if ev.event == app.InboxMailReceived {
					t.Fatal("stream delivered mail.received after the authorizing credential was revoked")
				}
			default:
			}
			return
		case <-deadline:
			t.Fatal("stream stayed open after the authorizing credential was revoked")
		}
	}
}
