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
