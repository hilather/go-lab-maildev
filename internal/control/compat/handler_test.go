package compat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/control/rest"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/preview"
)

var ulidRe = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

func TestEmptyListIsArray(t *testing.T) {
	h, _ := newTestHandler(t)
	got := doReq(t, h, http.MethodGet, "/email", "")
	requireStatus(t, got, http.StatusOK)
	if got.Body.String() != "[]" {
		t.Fatalf("list=%s", got.Body.String())
	}
}

func TestListOmitsBodies(t *testing.T) {
	h, svc := newTestHandler(t)
	insertMIME(t, svc, "simple-text.eml", model.Envelope{}, fixtureTime())
	got := doReq(t, h, http.MethodGet, "/email", "")
	requireStatus(t, got, http.StatusOK)
	items := decodeArray(t, got)
	if len(items) != 1 {
		t.Fatalf("len=%d body=%s", len(items), got.Body.String())
	}
	if items[0]["text"] != "" || items[0]["html"] != "" {
		t.Fatalf("list must omit bodies: %s", got.Body.String())
	}
	if items[0]["subject"] != "hello lab" {
		t.Fatalf("subject=%v", items[0]["subject"])
	}
	if _, ok := items[0]["stream"]; ok {
		t.Fatal("list must not leak stream")
	}
}

func TestListTextPrefix(t *testing.T) {
	h, svc := newTestHandler(t)
	insertMIME(t, svc, "simple-text.eml", model.Envelope{}, fixtureTime())
	got := doReq(t, h, http.MethodGet, "/email?text=1", "")
	requireStatus(t, got, http.StatusOK)
	items := decodeArray(t, got)
	text, _ := items[0]["text"].(string)
	if !strings.Contains(text, "plain body line") {
		t.Fatalf("text prefix=%q", text)
	}
	if items[0]["html"] != "" {
		t.Fatal("html must stay empty on list")
	}
}

func TestGetMarksRead(t *testing.T) {
	h, svc := newTestHandler(t)
	id := insertMIME(t, svc, "simple-text.eml", model.Envelope{}, fixtureTime())
	got := doReq(t, h, http.MethodGet, "/email/"+id, "")
	requireStatus(t, got, http.StatusOK)
	m := decodeJSON(t, got)
	if m["read"] != true {
		t.Fatalf("compat GET must mark read: %s", got.Body.String())
	}
	if !strings.Contains(m["text"].(string), "plain body line") {
		t.Fatalf("get must include body: %s", got.Body.String())
	}
	again, err := svc.GetMessage(context.Background(), app.Actor{ID: "t"}, id, false)
	if err != nil || !again.Read {
		t.Fatalf("store read=%v err=%v", again, err)
	}
}

func TestRelayAlways403(t *testing.T) {
	h, svc := newTestHandler(t)
	id := insertSubject(t, svc, "relay-me", "nope")
	for _, path := range []string{
		"/email/" + id + "/relay",
		"/email/" + id + "/relay/other@lab.test",
		"/email/missing/relay",
	} {
		got := doReq(t, h, http.MethodPost, path, `{}`)
		p := requireProblem(t, got, http.StatusForbidden, "receive_only")
		if !strings.Contains(p["detail"].(string), "receive-only") {
			t.Fatalf("detail=%v", p["detail"])
		}
		if got.Code == http.StatusOK {
			t.Fatal("relay must not look like success")
		}
	}
	still := doReq(t, h, http.MethodGet, "/email", "")
	if len(decodeArray(t, still)) != 1 {
		t.Fatal("relay must be a no-op")
	}
}

func TestAttachmentNamedRelayDownloads(t *testing.T) {
	h, svc := newTestHandler(t)
	res, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), &model.Message{
		Subject: "relay-name",
		Attachments: []model.Attachment{{
			Filename:    "relay.pdf",
			ContentType: "application/pdf",
			Data:        []byte("%PDF-1.4"),
		}},
		Raw: []byte("Subject: relay-name\r\n\r\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := doReq(t, h, http.MethodGet, "/email/"+res.ID+"/attachment/relay.pdf", "")
	requireStatus(t, got, http.StatusOK)
	if got.Body.String() != "%PDF-1.4" {
		t.Fatalf("body=%q", got.Body.String())
	}
	if strings.Contains(got.Header().Get("Content-Type"), "problem") {
		t.Fatal("filename relay.pdf must not be receive_only")
	}
}

func TestHealthzAndConfig(t *testing.T) {
	h, _ := newTestHandler(t)
	hz := doReq(t, h, http.MethodGet, "/healthz", "")
	requireStatus(t, hz, http.StatusOK)
	if decodeJSON(t, hz)["status"] != "ok" {
		t.Fatalf("healthz=%s", hz.Body.String())
	}

	down, _ := newTestHandlerReady(t, false)
	bad := doReq(t, down, http.MethodGet, "/healthz", "")
	requireStatus(t, bad, http.StatusServiceUnavailable)

	cfg := doReq(t, h, http.MethodGet, "/config", "")
	requireStatus(t, cfg, http.StatusOK)
	m := decodeJSON(t, cfg)
	if m["receiveOnly"] != true {
		t.Fatalf("receiveOnly=%v", m["receiveOnly"])
	}
	if m["hostname"] != "labmail.lab" {
		t.Fatalf("hostname=%v", m["hostname"])
	}
	smtp, _ := m["smtp"].(map[string]any)
	web, _ := m["web"].(map[string]any)
	if smtp["address"] != ":1025" || web["address"] != ":1080" {
		t.Fatalf("config=%s", cfg.Body.String())
	}
	if _, ok := m["outgoingHost"]; ok {
		t.Fatal("must not clone maildev outgoingHost")
	}
}

func TestUnauthenticatedIsNot401(t *testing.T) {
	h, _ := newTestHandler(t)
	got := doReq(t, h, http.MethodGet, "/email", "")
	if got.Code == http.StatusUnauthorized {
		t.Fatal("COMPAT-001 must not claim 401; TestMaildevScenarioCompat is PR 9")
	}
	requireStatus(t, got, http.StatusOK)
}

func TestFakePrincipalInjector(t *testing.T) {
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: copyDefaults(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	var seen app.Actor
	h, err := New(Config{
		Service: svc,
		Ready:   func() bool { return true },
		Principal: func(r *http.Request) app.Actor {
			seen = stubPrincipal(r)
			return seen
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/email", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusOK)
	if seen.ID != "basic" || seen.Transport != "compat" {
		t.Fatalf("principal=%+v", seen)
	}
}

func TestFilterAndSkip(t *testing.T) {
	h, svc := newTestHandler(t)
	insertMIME(t, svc, "simple-text.eml", model.Envelope{}, fixtureTime())
	insertSubject(t, svc, "other", "x")

	hit := doReq(t, h, http.MethodGet, "/email?subject=hello+lab", "")
	requireStatus(t, hit, http.StatusOK)
	if len(decodeArray(t, hit)) != 1 {
		t.Fatalf("subject filter=%s", hit.Body.String())
	}

	dotted := doReq(t, h, http.MethodGet, "/email?headers.to=Bob+%3Cbob@lab.test%3E", "")
	requireStatus(t, dotted, http.StatusOK)
	if len(decodeArray(t, dotted)) != 1 {
		t.Fatalf("headers.to filter=%s", dotted.Body.String())
	}

	addr := doReq(t, h, http.MethodGet, "/email?from.address=alice@lab.test", "")
	requireStatus(t, addr, http.StatusOK)
	if len(decodeArray(t, addr)) != 1 {
		t.Fatalf("from.address filter=%s", addr.Body.String())
	}

	skip := doReq(t, h, http.MethodGet, "/email?skip=1", "")
	requireStatus(t, skip, http.StatusOK)
	if len(decodeArray(t, skip)) != 1 {
		t.Fatalf("skip=%s", skip.Body.String())
	}
	none := doReq(t, h, http.MethodGet, "/email?skip=5", "")
	if none.Body.String() != "[]" {
		t.Fatalf("skip past end=%s", none.Body.String())
	}

	unread := doReq(t, h, http.MethodGet, "/email?read=false", "")
	requireStatus(t, unread, http.StatusOK)
	if len(decodeArray(t, unread)) != 2 {
		t.Fatalf("read=false=%s", unread.Body.String())
	}
	seen := doReq(t, h, http.MethodGet, "/email?read=true", "")
	if seen.Body.String() != "[]" {
		t.Fatalf("read=true before get=%s", seen.Body.String())
	}
}

func TestDeleteAndHTMLAndAttachment(t *testing.T) {
	h, svc := newTestHandler(t)
	id := insertMIME(t, svc, "attachment-base64.eml", model.Envelope{}, fixtureTime())

	html := doReq(t, h, http.MethodGet, "/email/"+id+"/html", "")
	requireStatus(t, html, http.StatusOK)
	if html.Header().Get("Content-Security-Policy") != preview.CSP {
		t.Fatalf("csp=%q", html.Header().Get("Content-Security-Policy"))
	}

	att := doReq(t, h, http.MethodGet, "/email/"+id+"/attachment/notes.txt", "")
	requireStatus(t, att, http.StatusOK)
	if !strings.Contains(att.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("disp=%s", att.Header().Get("Content-Disposition"))
	}
	if att.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}

	missing := doReq(t, h, http.MethodGet, "/email/"+id+"/attachment/nope.txt", "")
	requireProblem(t, missing, http.StatusNotFound, "not_found")

	del := doReq(t, h, http.MethodDelete, "/email/"+id, "")
	requireStatus(t, del, http.StatusOK)
	if del.Body.String() != "true" {
		t.Fatalf("delete=%s", del.Body.String())
	}
	insertSubject(t, svc, "leftover", "x")
	all := doReq(t, h, http.MethodDelete, "/email/all", "")
	requireStatus(t, all, http.StatusOK)
	if doReq(t, h, http.MethodGet, "/email", "").Body.String() != "[]" {
		t.Fatal("clear left messages")
	}
}

func TestGoldensSubjectFromToAndAttachment(t *testing.T) {
	h, svc := newTestHandler(t)
	env := model.Envelope{From: "alice@lab.test", To: []string{"bob@lab.test"}, HELO: "client.example", RemoteAddr: "10.42.0.9"}
	id := insertMIME(t, svc, "simple-text.eml", env, fixtureTime())
	got := doReq(t, h, http.MethodGet, "/email/"+id, "")
	requireStatus(t, got, http.StatusOK)
	item := decodeJSON(t, got)
	if !ulidRe.MatchString(item["id"].(string)) {
		t.Fatalf("id must be ULID, not maildev 8-char: %v", item["id"])
	}
	if item["time"] != "2026-08-17T12:00:00.000Z" {
		t.Fatalf("time=%v", item["time"])
	}
	if item["read"] != true {
		t.Fatal("get marks read")
	}

	wantItem := readGolden(t, "email-item.golden.json")
	if item["subject"] != wantItem["subject"] || item["messageId"] != wantItem["messageId"] || item["priority"] != wantItem["priority"] {
		t.Fatalf("item fields subject/messageId/priority got=%v want=%v", item, wantItem)
	}
	requireAddr(t, item["from"], "alice@lab.test", "Alice")
	requireAddr(t, item["to"], "bob@lab.test", "Bob")
	// List read bit stays false until GET :id; compare against the unread golden.
	list := decodeArray(t, doReq(t, h, http.MethodGet, "/email", ""))
	if list[0]["read"] != true {
		t.Fatal("after GET, list should show read")
	}

	attID := insertMIME(t, svc, "attachment-base64.eml", env, fixtureTime())
	attGot := doReq(t, h, http.MethodGet, "/email/"+attID, "")
	requireStatus(t, attGot, http.StatusOK)
	atts, _ := decodeJSON(t, attGot)["attachments"].([]any)
	if len(atts) != 1 {
		t.Fatalf("attachments=%s", attGot.Body.String())
	}
	att := atts[0].(map[string]any)
	if _, ok := att["stream"]; ok {
		t.Fatal("attachment must omit maildev stream")
	}
	wantAtt := readGolden(t, "email-attachment.golden.json")
	for _, key := range []string{"fileName", "contentType", "contentDisposition", "checksum"} {
		if att[key] != wantAtt[key] {
			t.Fatalf("%s=%v want %v", key, att[key], wantAtt[key])
		}
	}
	sum, _ := att["checksum"].(string)
	if len(sum) != 64 {
		t.Fatalf("checksum must be sha256 hex, not md5: %q", sum)
	}
}

func TestWiredOnManagementListener(t *testing.T) {
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: copyDefaults(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	ch, err := New(Config{Service: svc, Ready: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rest.New(rest.Config{Service: svc, RatePerSec: -1, Mounts: ch.Mounts(), Ready: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	h := rs.Handler()
	requireStatus(t, doReq(t, h, http.MethodGet, "/v1/health/ready", ""), http.StatusOK)
	requireStatus(t, doReq(t, h, http.MethodGet, "/healthz", ""), http.StatusOK)
	list := doReq(t, h, http.MethodGet, "/email", "")
	requireStatus(t, list, http.StatusOK)
	if list.Body.String() != "[]" {
		t.Fatalf("email=%s", list.Body.String())
	}
	requireProblem(t, doReq(t, h, http.MethodPost, "/email/x/relay", `{}`), http.StatusForbidden, "receive_only")

	res, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), &model.Message{
		Subject: "wired-relay-name",
		Attachments: []model.Attachment{{
			Filename:    "relay.pdf",
			ContentType: "application/pdf",
			Data:        []byte("%PDF"),
		}},
		Raw: []byte("Subject: wired-relay-name\r\n\r\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	att := doReq(t, h, http.MethodGet, "/email/"+res.ID+"/attachment/relay.pdf", "")
	requireStatus(t, att, http.StatusOK)
}

func TestOriginAndMethod(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/email", nil)
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	requireProblem(t, rec, http.StatusForbidden, "forbidden")

	got := doReq(t, h, http.MethodPost, "/email", "")
	requireProblem(t, got, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestHTMLRewritesCIDAndMarksRead(t *testing.T) {
	h, svc := newTestHandler(t)
	id := insertMIME(t, svc, "html-inline-cid.eml", model.Envelope{}, fixtureTime())
	got := doReq(t, h, http.MethodGet, "/email/"+id+"/html", "")
	requireStatus(t, got, http.StatusOK)
	if got.Header().Get("Content-Security-Policy") != preview.CSP {
		t.Fatalf("csp=%q", got.Header().Get("Content-Security-Policy"))
	}
	if strings.Contains(got.Body.String(), "cid:") {
		t.Fatalf("cid not rewritten: %s", got.Body.String())
	}
	if !strings.Contains(got.Body.String(), "data:image/png;base64,") {
		t.Fatalf("missing data url: %s", got.Body.String())
	}
	again, err := svc.GetMessage(context.Background(), app.Actor{ID: "t"}, id, false)
	if err != nil || !again.Read {
		t.Fatalf("html get must mark read: read=%v err=%v", again, err)
	}
}

func TestTextPrefixRuneBoundary(t *testing.T) {
	got := prefixBytes("éééé", 3)
	if !utf8.ValidString(got) || got != "é" {
		t.Fatalf("prefix=%q", got)
	}
}

func TestMissingMessage(t *testing.T) {
	h, _ := newTestHandler(t)
	requireProblem(t, doReq(t, h, http.MethodGet, "/email/01AAAAAAAAAAAAAAAAAAAAAAAA", ""), http.StatusNotFound, "not_found")
}

func requireAddr(t *testing.T, raw any, address, name string) {
	t.Helper()
	list, ok := raw.([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("addrs=%v", raw)
	}
	m, ok := list[0].(map[string]any)
	if !ok || m["address"] != address || m["name"] != name {
		t.Fatalf("addr=%v want %s %q", raw, address, name)
	}
}

func readGolden(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "compat", name))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
