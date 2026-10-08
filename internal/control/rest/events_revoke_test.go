package rest

import (
	"bufio"
	"context"
	"encoding/json"
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

// TestSSEStopsWhenSessionRevoked: a cookie session that is deleted must stop
// receiving inbox events. Authorization is checked only at connect today.
func TestSSEStopsWhenSessionRevoked(t *testing.T) {
	s, svc, _ := newAuthServer(t)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	loginReq, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	loginReq.Header.Set("Authorization", "Bearer "+testBearerToken)
	loginRes, err := http.DefaultClient.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	loginBody, _ := io.ReadAll(loginRes.Body)
	_ = loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, loginBody)
	}
	var created struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(loginBody, &created); err != nil || created.CSRF == "" {
		t.Fatalf("login body=%s", loginBody)
	}
	var cookie *http.Cookie
	for _, c := range loginRes.Cookies() {
		if c.Name == auth.CookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("missing session cookie")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamReq.AddCookie(cookie)
	streamRes, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = streamRes.Body.Close() }()
	if streamRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(streamRes.Body)
		t.Fatalf("stream status=%d body=%s", streamRes.StatusCode, b)
	}

	frames := make(chan string, 8)
	go func() {
		br := bufio.NewReader(streamRes.Body)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				close(frames)
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(line, "event: ") {
				select {
				case frames <- strings.TrimPrefix(line, "event: "):
				default:
				}
			}
		}
	}()

	delReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	delReq.AddCookie(cookie)
	delReq.Header.Set(auth.CSRFHeader, created.CSRF)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(delRes.Body)
	_ = delRes.Body.Close()
	if delRes.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status=%d", delRes.StatusCode)
	}

	insertMail(t, svc, "after-revoke", "secret subject")
	deadline := time.After(750 * time.Millisecond)
	for {
		select {
		case ev, ok := <-frames:
			if !ok {
				return
			}
			if ev == app.InboxMailReceived {
				t.Fatal("revoked session still received mail.received")
			}
		case <-deadline:
			return
		}
	}
}

// TestEventsStreamLiveCookieKeepsDelivering rechecks a live cookie across one
// heartbeat, still delivers mail.received, then ends the stream after logout.
func TestEventsStreamLiveCookieKeepsDelivering(t *testing.T) {
	s, svc, _ := newAuthServer(t)
	s.sseHeartbeat = 25 * time.Millisecond
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	loginReq, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	loginReq.Header.Set("Authorization", "Bearer "+testBearerToken)
	loginRes, err := http.DefaultClient.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	loginBody, _ := io.ReadAll(loginRes.Body)
	_ = loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRes.StatusCode, loginBody)
	}
	var created struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(loginBody, &created); err != nil || created.CSRF == "" {
		t.Fatalf("login body=%s", loginBody)
	}
	var cookie *http.Cookie
	for _, c := range loginRes.Cookies() {
		if c.Name == auth.CookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("missing session cookie")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamReq.AddCookie(cookie)
	streamRes, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = streamRes.Body.Close() }()
	if streamRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(streamRes.Body)
		t.Fatalf("stream status=%d body=%s", streamRes.StatusCode, b)
	}
	frames, heartbeats, done := readSSE(streamRes.Body)
	select {
	case <-heartbeats:
	case <-done:
		t.Fatal("stream closed before the live-session heartbeat recheck")
	case <-time.After(time.Second):
		t.Fatal("missing heartbeat")
	}
	insertMail(t, svc, "while-live", "visible")
	if ev := waitSSE(t, frames, done, app.InboxMailReceived); ev.data["subject"] != "while-live" {
		t.Fatalf("received=%+v", ev)
	}

	delReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	delReq.AddCookie(cookie)
	delReq.Header.Set(auth.CSRFHeader, created.CSRF)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(delRes.Body)
	_ = delRes.Body.Close()
	if delRes.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status=%d", delRes.StatusCode)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream stayed open after the cookie session was deleted")
	}
}

// TestEventsStreamLoopbackIgnoresStaleCookie: a stale cookie on a dev-loopback
// stream missed at connect, so heartbeat and mail.received keep flowing.
func TestEventsStreamLoopbackIgnoresStaleCookie(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: dev-loopback-unauth\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(t.Context(), app.Options{BootstrapPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: v, RatePerSec: -1, SSEHeartbeat: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamReq.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "stale-or-expired"})
	streamRes, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = streamRes.Body.Close() }()
	if streamRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(streamRes.Body)
		t.Fatalf("stream status=%d body=%s", streamRes.StatusCode, b)
	}
	frames, heartbeats, done := readSSE(streamRes.Body)
	select {
	case <-heartbeats:
	case <-done:
		t.Fatal("stale cookie closed the loopback stream on heartbeat")
	case <-time.After(time.Second):
		t.Fatal("missing heartbeat")
	}
	insertMail(t, svc, "loopback-stale", "body")
	ev := waitSSE(t, frames, done, app.InboxMailReceived)
	if ev.data["subject"] != "loopback-stale" {
		t.Fatalf("received=%+v", ev)
	}
}

func readSSE(body io.Reader) (chan sseTestFrame, chan struct{}, chan struct{}) {
	frames := make(chan sseTestFrame, 8)
	heartbeats := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		br := bufio.NewReader(body)
		var ev string
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case line == ": heartbeat":
				select {
				case heartbeats <- struct{}{}:
				default:
				}
			case strings.HasPrefix(line, "event: "):
				ev = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				var payload map[string]any
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload)
				select {
				case frames <- sseTestFrame{event: ev, data: payload}:
				default:
				}
				ev = ""
			}
		}
	}()
	return frames, heartbeats, done
}

func waitSSE(t *testing.T, frames <-chan sseTestFrame, done <-chan struct{}, name string) sseTestFrame {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-done:
			t.Fatalf("stream closed while waiting for %s", name)
		case f := <-frames:
			if f.event == name {
				return f
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", name)
		}
	}
}
