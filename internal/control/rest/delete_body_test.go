package rest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDeleteHonorsGenerationBodyWithoutContentType: a JSON body that carries
// expectedStoreGeneration is the conditional-delete guard. Omitting
// Content-Type must not drop that guard and delete anyway.
func TestDeleteHonorsGenerationBodyWithoutContentType(t *testing.T) {
	s, svc := newTestServer(t)
	id := insertMail(t, svc, "cond", "body")
	gen := svc.Inbox().Generation()
	body := fmt.Sprintf(`{"expectedStoreGeneration":%d}`, gen+1000)
	req := httptest.NewRequest(http.MethodDelete, "/v1/messages/"+id, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Del("Content-Type")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	requireProblem(t, rec, http.StatusConflict, "revision_conflict")
}

// TestClearHonorsGenerationBodyWithoutContentType: DELETE /v1/messages uses
// the same body. A stale generation is 409 and the message stays.
func TestClearHonorsGenerationBodyWithoutContentType(t *testing.T) {
	s, svc := newTestServer(t)
	id := insertMail(t, svc, "cond", "body")
	gen := svc.Inbox().Generation()
	body := fmt.Sprintf(`{"expectedStoreGeneration":%d}`, gen+1000)
	req := httptest.NewRequest(http.MethodDelete, "/v1/messages", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Del("Content-Type")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	requireProblem(t, rec, http.StatusConflict, "revision_conflict")
	list := doReq(t, s.Handler(), http.MethodGet, "/v1/messages", "")
	if !strings.Contains(list.Body.String(), id) {
		t.Fatalf("clear removed %s despite revision_conflict: %s", id, list.Body.String())
	}
}
