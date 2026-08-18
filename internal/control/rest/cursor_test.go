package rest

import (
	"net/http"
	"testing"
)

func TestCursorHMACAndStale(t *testing.T) {
	s, svc := newTestServer(t)
	h := s.Handler()
	for i := 0; i < 3; i++ {
		insertMail(t, svc, "n", "b")
	}
	page := doReq(t, h, http.MethodGet, "/v1/messages?limit=1", "")
	requireStatus(t, page, http.StatusOK)
	cur, _ := decodeJSON(t, page)["nextCursor"].(string)
	if cur == "" {
		t.Fatal("missing nextCursor")
	}
	page2 := doReq(t, h, http.MethodGet, "/v1/messages?limit=1&cursor="+cur, "")
	requireStatus(t, page2, http.StatusOK)

	insertMail(t, svc, "new", "x")
	stale := doReq(t, h, http.MethodGet, "/v1/messages?limit=1&cursor="+cur, "")
	p := requireProblem(t, stale, http.StatusBadRequest, "cursor_stale")
	if p["code"] == "validation_failed" {
		t.Fatal("cursor_stale must not wrap as validation_failed")
	}

	bad := doReq(t, h, http.MethodGet, "/v1/messages?cursor=not-a-cursor", "")
	requireProblem(t, bad, http.StatusBadRequest, "validation_failed")
}

func TestCursorRotatesOnReset(t *testing.T) {
	s, svc := newTestServer(t)
	h := s.Handler()
	insertMail(t, svc, "a", "1")
	insertMail(t, svc, "b", "2")
	page := doReq(t, h, http.MethodGet, "/v1/messages?limit=1", "")
	cur, _ := decodeJSON(t, page)["nextCursor"].(string)
	reset := doReq(t, h, http.MethodPost, "/v1/state:reset", `{}`)
	requireStatus(t, reset, http.StatusOK)
	got := doReq(t, h, http.MethodGet, "/v1/messages?cursor="+cur, "")
	requireProblem(t, got, http.StatusBadRequest, "cursor_stale")
}
