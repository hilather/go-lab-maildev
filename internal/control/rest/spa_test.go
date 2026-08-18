package rest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/web"
)

func TestSPAFallbackWhenEnabled(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }

	got := doReq(t, s.Handler(), http.MethodGet, "/", "")
	requireStatus(t, got, http.StatusOK)
	if !strings.Contains(got.Body.String(), "LabMail") {
		t.Fatalf("GET / body=%s", got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET / content-type=%q", ct)
	}

	inbox := doReq(t, s.Handler(), http.MethodGet, "/messages/01JTEST", "")
	requireStatus(t, inbox, http.StatusOK)
	if !strings.Contains(inbox.Body.String(), "LabMail") {
		t.Fatalf("SPA fallback body=%s", inbox.Body.String())
	}
}

func TestSPADisabledIs404(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return false }

	got := doReq(t, s.Handler(), http.MethodGet, "/", "")
	requireProblem(t, got, http.StatusNotFound, "not_found")
}

func TestSPADoesNotCaptureAPI(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }

	got := doReq(t, s.Handler(), http.MethodGet, "/v1/does-not-exist", "")
	requireProblem(t, got, http.StatusNotFound, "not_found")
	if strings.Contains(got.Body.String(), "<!doctype") {
		t.Fatal("API miss served HTML")
	}
}
