package compat

import (
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
	"github.com/hilather/go-lab-maildev/internal/model"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func copyDefaults(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "labmail.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestHandler(t *testing.T) (*Handler, *app.App) {
	t.Helper()
	return newTestHandlerReady(t, true)
}

func newTestHandlerReady(t *testing.T, ready bool) (*Handler, *app.App) {
	t.Helper()
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: copyDefaults(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	h, err := New(Config{
		Service: svc,
		Ready:   func() bool { return ready },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, svc
}

func doReq(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.RemoteAddr = "127.0.0.1:54321"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	return out
}

func decodeArray(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("array json: %v body=%s", err, rec.Body.String())
	}
	return out
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status=%d want %d body=%s", rec.Code, want, rec.Body.String())
	}
}

func requireProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) map[string]any {
	t.Helper()
	requireStatus(t, rec, status)
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	m := decodeJSON(t, rec)
	if got, _ := m["code"].(string); got != code {
		t.Fatalf("code=%v want %s body=%s", m["code"], code, rec.Body.String())
	}
	return m
}

func insertMIME(t *testing.T, svc *app.App, name string, env model.Envelope, at time.Time) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "mime", name))
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), &model.Message{
		Raw:        raw,
		Envelope:   env,
		ReceivedAt: at,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.ID
}

func insertSubject(t *testing.T, svc *app.App, subject, body string) string {
	t.Helper()
	raw := []byte("Subject: " + subject + "\r\n\r\n" + body + "\r\n")
	res, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), &model.Message{Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	return res.ID
}

func fixtureTime() time.Time {
	return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
}
