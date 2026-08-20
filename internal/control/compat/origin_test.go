package compat

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompatOriginRules(t *testing.T) {
	h, _ := newTestHandler(t)

	missing := httptest.NewRequest(http.MethodGet, "/email", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, missing)
	requireStatus(t, rec, http.StatusOK)

	loop := httptest.NewRequest(http.MethodGet, "/email", nil)
	loop.Header.Set("Origin", "http://127.0.0.1:1080")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, loop)
	requireStatus(t, rec, http.StatusOK)

	evil := httptest.NewRequest(http.MethodGet, "/email", nil)
	evil.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, evil)
	requireProblem(t, rec, http.StatusForbidden, "forbidden")

	file := httptest.NewRequest(http.MethodGet, "/email", nil)
	file.Header.Set("Origin", "file://localhost")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, file)
	requireProblem(t, rec, http.StatusForbidden, "forbidden")
}

func TestCompatOriginStar(t *testing.T) {
	h, _ := newTestHandler(t)
	h.cfg.AllowedOrigins = []string{"*"}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusOK)
}

func TestCompatOriginPrivate(t *testing.T) {
	h, _ := newTestHandler(t)
	h.cfg.AllowedOrigins = []string{"private"}

	ok := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	ok.Header.Set("Origin", "http://192.168.1.9:1080")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, ok)
	requireStatus(t, rec, http.StatusOK)

	evil := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	evil.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, evil)
	requireProblem(t, rec, http.StatusForbidden, "forbidden")
}

func TestCompatOptionsCORSDisabledWithStar(t *testing.T) {
	h, _ := newTestHandler(t)
	h.cfg.AllowedOrigins = []string{"*"}

	req := httptest.NewRequest(http.MethodOptions, "/email", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	m := requireProblem(t, rec, http.StatusForbidden, "forbidden")
	if d, _ := m["detail"].(string); d != "CORS is disabled" {
		t.Fatalf("detail=%q", d)
	}
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("Access-Control-Allow-Origin=%q", v)
	}
}
