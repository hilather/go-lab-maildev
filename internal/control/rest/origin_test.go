package rest

import (
	"net/http"
	"testing"
)

func TestOriginAllowlist(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()

	missing := httptestReq(http.MethodGet, "/v1/health/live", "")
	requireStatus(t, doRaw(h, missing), http.StatusOK)

	req := httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://127.0.0.1:1080")
	requireStatus(t, doRaw(h, req), http.StatusOK)

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "https://evil.example")
	requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "file://localhost")
	requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")

	s.cfg.AllowedOrigins = []string{"http://192.168.1.9:1080"}
	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	requireStatus(t, doRaw(h, req), http.StatusOK)
}

func TestOriginStar(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.AllowedOrigins = []string{"*"}
	h := s.Handler()

	req := httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "https://evil.example")
	requireStatus(t, doRaw(h, req), http.StatusOK)

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	requireStatus(t, doRaw(h, req), http.StatusOK)

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "file://localhost")
	requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "null")
	requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")
}

func TestOriginPrivate(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.AllowedOrigins = []string{"private"}
	h := s.Handler()

	req := httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	requireStatus(t, doRaw(h, req), http.StatusOK)

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "https://evil.example")
	requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://100.64.0.1:1080")
	requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")
}

func TestOptionsCORSDisabledWithStar(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.AllowedOrigins = []string{"*"}
	h := s.Handler()

	req := httptestReq(http.MethodOptions, "/v1/health/live", "")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	rec := doRaw(h, req)
	m := requireProblem(t, rec, http.StatusForbidden, "forbidden")
	if d, _ := m["detail"].(string); d != "CORS is disabled" {
		t.Fatalf("detail=%q", d)
	}
	requireNoACAO(t, rec)

	loop := httptestReq(http.MethodOptions, "/v1/health/live", "")
	loop.Header.Set("Origin", "http://127.0.0.1:1080")
	lrec := doRaw(h, loop)
	lm := requireProblem(t, lrec, http.StatusForbidden, "forbidden")
	if d, _ := lm["detail"].(string); d != "CORS is disabled" {
		t.Fatalf("loopback OPTIONS detail=%q", d)
	}
	requireNoACAO(t, lrec)
}
