package rest

import (
	"net/http"
	"testing"
)

func TestOriginAllowlist(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()

	req := httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://localhost:1080")
	rec := doRaw(h, req)
	requireStatus(t, rec, http.StatusOK)

	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	rec = doRaw(h, req)
	requireProblem(t, rec, http.StatusForbidden, "forbidden")

	s.cfg.AllowedOrigins = []string{"http://192.168.1.9:1080"}
	req = httptestReq(http.MethodGet, "/v1/health/live", "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	rec = doRaw(h, req)
	requireStatus(t, rec, http.StatusOK)
}
