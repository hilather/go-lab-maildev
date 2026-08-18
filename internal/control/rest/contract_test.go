package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestRoutesRegisteredFromRegistry(t *testing.T) {
	s, _ := newTestServer(t)
	if len(s.routes) == 0 {
		t.Fatal("no routes")
	}
	seen := map[string]bool{}
	for _, rt := range s.routes {
		seen[rt.method+" "+rt.path] = true
	}
	for _, c := range capabilities.All() {
		for _, b := range c.REST {
			ref := strings.ToUpper(b.Method) + " " + b.Path
			if !seen[ref] {
				t.Errorf("missing registry route %s", ref)
			}
		}
	}
}

func TestContractReads(t *testing.T) {
	s, svc := newTestServer(t)
	h := s.Handler()

	live := doReq(t, h, http.MethodGet, "/v1/health/live", "")
	requireStatus(t, live, http.StatusOK)
	if decodeJSON(t, live)["status"] != "ok" {
		t.Fatalf("live=%s", live.Body.String())
	}

	ready := doReq(t, h, http.MethodGet, "/v1/health/ready", "")
	requireStatus(t, ready, http.StatusOK)

	ver := doReq(t, h, http.MethodGet, "/v1/version", "")
	requireStatus(t, ver, http.StatusOK)
	if decodeJSON(t, ver)["protocols"] == nil {
		t.Fatalf("version=%s", ver.Body.String())
	}

	caps := doReq(t, h, http.MethodGet, "/v1/capabilities", "")
	requireStatus(t, caps, http.StatusOK)
	clist, _ := decodeJSON(t, caps)["capabilities"].([]any)
	if len(clist) == 0 {
		t.Fatal("empty capabilities")
	}

	st := doReq(t, h, http.MethodGet, "/v1/status", "")
	requireStatus(t, st, http.StatusOK)
	if decodeJSON(t, st)["revisions"] == nil {
		t.Fatalf("status=%s", st.Body.String())
	}

	schema := doReq(t, h, http.MethodGet, "/v1/schema/config", "")
	requireStatus(t, schema, http.StatusOK)
	if !strings.Contains(schema.Body.String(), "labmail.dev/v1alpha1") {
		t.Fatalf("schema missing api version")
	}

	state := doReq(t, h, http.MethodGet, "/v1/state", "")
	requireStatus(t, state, http.StatusOK)
	if decodeJSON(t, state)["runtimeRevision"] == "" {
		t.Fatalf("state=%s", state.Body.String())
	}

	id := insertMail(t, svc, "hello", "see https://app.lab.test/verify?token=abc1 Your code is 123456")
	list := doReq(t, h, http.MethodGet, "/v1/messages?limit=1", "")
	requireStatus(t, list, http.StatusOK)
	lm := decodeJSON(t, list)
	if lm["storeGeneration"] == nil {
		t.Fatalf("list=%s", list.Body.String())
	}
	items, _ := lm["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items=%s", list.Body.String())
	}
	item := items[0].(map[string]any)
	if _, ok := item["text"]; ok {
		t.Fatal("list item must omit text")
	}
	if item["hasHTML"] == nil {
		t.Fatal("list item missing hasHTML")
	}

	got := doReq(t, h, http.MethodGet, "/v1/messages/"+id, "")
	requireStatus(t, got, http.StatusOK)
	gm := decodeJSON(t, got)
	if gm["text"] == nil || gm["read"] != false {
		t.Fatalf("get default markRead=false: %s", got.Body.String())
	}

	marked := doReq(t, h, http.MethodGet, "/v1/messages/"+id+"?markRead=true", "")
	requireStatus(t, marked, http.StatusOK)
	if decodeJSON(t, marked)["read"] != true {
		t.Fatal("markRead=true")
	}

	raw := doReq(t, h, http.MethodGet, "/v1/messages/"+id+"/raw", "")
	requireStatus(t, raw, http.StatusOK)
	if !strings.Contains(raw.Header().Get("Content-Type"), "message/rfc822") {
		t.Fatalf("raw type=%s", raw.Header().Get("Content-Type"))
	}

	missing := doReq(t, h, http.MethodGet, "/v1/messages/01AAAAAAAAAAAAAAAAAAAAAAAA", "")
	requireProblem(t, missing, http.StatusNotFound, "not_found")
}

func TestContractMutations(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	st := doReq(t, h, http.MethodGet, "/v1/state", "")
	rev := decodeJSON(t, st)["runtimeRevision"].(string)

	body, err := json.Marshal(map[string]any{
		"expectedRevision": rev,
		"reason":           "hide SIZE",
		"operations": []model.Operation{{
			Op:             model.OpReplaceHideExtensions,
			HideExtensions: []string{"SIZE"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	val := doReq(t, h, http.MethodPost, "/v1/state:validate", `{"operations":[{"op":"replaceHideExtensions","hideExtensions":["SIZE"]}]}`)
	requireStatus(t, val, http.StatusOK)

	plan := doReq(t, h, http.MethodPost, "/v1/changes:plan", string(body))
	requireStatus(t, plan, http.StatusOK)
	if decodeJSON(t, plan)["candidateRevision"] == rev {
		t.Fatal("plan did not change revision")
	}

	bad := doReq(t, h, http.MethodPost, "/v1/changes:apply", `{"expectedRevision":"sha256:deadbeef","operations":[{"op":"replaceHideExtensions","hideExtensions":["SIZE"]}]}`)
	requireProblem(t, bad, http.StatusConflict, "revision_conflict")

	apply := doReq(t, h, http.MethodPost, "/v1/changes:apply", string(body))
	requireStatus(t, apply, http.StatusOK)
	if decodeJSON(t, apply)["applied"] != true {
		t.Fatalf("apply=%s", apply.Body.String())
	}

	exp := doReq(t, h, http.MethodGet, "/v1/state:export?format=yaml", "")
	requireStatus(t, exp, http.StatusOK)
	if !strings.Contains(exp.Header().Get("Content-Type"), "yaml") {
		t.Fatalf("export type=%s", exp.Header().Get("Content-Type"))
	}

	reset := doReq(t, h, http.MethodPost, "/v1/state:reset", `{"reason":"test"}`)
	requireStatus(t, reset, http.StatusOK)
}

func TestStoreOverNewCapCode(t *testing.T) {
	s, svc := newTestServer(t)
	h := s.Handler()
	insertMail(t, svc, "one", "a")
	insertMail(t, svc, "two", "b")
	rev := decodeJSON(t, doReq(t, h, http.MethodGet, "/v1/state", ""))["runtimeRevision"].(string)
	body, err := json.Marshal(map[string]any{
		"expectedRevision": rev,
		"operations": []map[string]any{{
			"op": "replaceStoreCaps",
			"store": map[string]any{
				"maxMessages": 1,
				"maxBytes":    "256MiB",
				"fullPolicy":  "reject",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := doReq(t, h, http.MethodPost, "/v1/changes:apply", string(body))
	p := requireProblem(t, got, http.StatusBadRequest, "store_over_new_cap")
	if p["code"] == "validation_failed" {
		t.Fatal("must not wrap store_over_new_cap")
	}
}

func TestWrongMethod(t *testing.T) {
	s, _ := newTestServer(t)
	got := doReq(t, s.Handler(), http.MethodPost, "/v1/health/live", "")
	requireProblem(t, got, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestRelayForbidden(t *testing.T) {
	s, _ := newTestServer(t)
	got := doReq(t, s.Handler(), http.MethodPost, "/v1/messages/x/relay", `{}`)
	requireProblem(t, got, http.StatusForbidden, "receive_only")
}

func TestSessionRegistered(t *testing.T) {
	s, _ := newTestServer(t)
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/session", "")
	requireStatus(t, got, http.StatusOK)
	if decodeJSON(t, got)["id"] == "" {
		t.Fatalf("session=%s", got.Body.String())
	}
}

func TestMarkReadDefaultFalse(t *testing.T) {
	s, svc := newTestServer(t)
	id := insertMail(t, svc, "unread", "body")
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/messages/"+id, "")
	requireStatus(t, got, http.StatusOK)
	if decodeJSON(t, got)["read"] != false {
		t.Fatal("native get must default markRead=false")
	}
	again, err := svc.GetMessage(context.Background(), app.Actor{ID: "t"}, id, false)
	if err != nil || again.Read {
		t.Fatalf("store read=%v err=%v", again, err)
	}
}

func TestDeleteIfMatchStoreGeneration(t *testing.T) {
	s, svc := newTestServer(t)
	id := insertMail(t, svc, "if-match", "b")
	gen := svc.Inbox().Generation()
	req := httptestReq(http.MethodDelete, "/v1/messages/"+id, "")
	req.Header.Set("If-Match", strconv.FormatUint(gen, 10))
	rec := doRaw(s.Handler(), req)
	requireStatus(t, rec, http.StatusNoContent)

	id = insertMail(t, svc, "if-match-bad", "b")
	req = httptestReq(http.MethodDelete, "/v1/messages/"+id, "")
	req.Header.Set("If-Match", "1")
	rec = doRaw(s.Handler(), req)
	requireProblem(t, rec, http.StatusConflict, "revision_conflict")
}

func TestWaitAndExtract(t *testing.T) {
	s, svc := newTestServer(t)
	h := s.Handler()
	insertMail(t, svc, "mcplab smoke", "Visit https://app.lab.test/ok Your code is 482193")
	wait := doReq(t, h, http.MethodPost, "/v1/messages:wait", `{"filter":{"subjectContains":"smoke"},"timeout":"1s"}`)
	requireStatus(t, wait, http.StatusOK)
	id := decodeJSON(t, wait)["id"].(string)
	ex := doReq(t, h, http.MethodPost, "/v1/messages/"+id+":extract", "")
	requireStatus(t, ex, http.StatusOK)
	em := decodeJSON(t, ex)
	urls, _ := em["urls"].([]any)
	if len(urls) != 1 || urls[0] != "https://app.lab.test/ok" {
		t.Fatalf("extract urls=%s", ex.Body.String())
	}
	timed := doReq(t, h, http.MethodPost, "/v1/messages:wait", `{"filter":{"subject":"never"},"timeout":"1ms"}`)
	requireProblem(t, timed, http.StatusGatewayTimeout, "timeout")
}
