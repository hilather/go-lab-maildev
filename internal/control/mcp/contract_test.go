package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsRegisteredFromRegistry(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	seen := map[string]bool{}
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		seen[tool.Name] = true
		if tool.InputSchema == nil {
			t.Errorf("%s missing input schema", tool.Name)
		}
	}
	want := capabilities.Tools()
	if len(seen) != len(want) {
		t.Errorf("live tools=%d registry=%d", len(seen), len(want))
	}
	for _, name := range want {
		if !seen[name] {
			t.Errorf("missing tool %s", name)
		}
	}
	for name := range seen {
		found := false
		for _, w := range want {
			if w == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("extra live tool %s", name)
		}
	}
	if seen["health.live"] || seen["mail_health_live"] {
		t.Fatal("health live must not be a tool")
	}
}

func TestResourcesRegisteredFromRegistry(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	seen := map[string]bool{}
	for r, err := range cs.Resources(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		seen[r.URI] = true
	}
	for tmpl, err := range cs.ResourceTemplates(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		seen[tmpl.URITemplate] = true
	}
	want := capabilities.Resources()
	if len(seen) != len(want) {
		t.Errorf("live resources=%d registry=%d", len(seen), len(want))
	}
	for _, uri := range want {
		if !seen[uri] {
			t.Errorf("missing resource %s", uri)
		}
	}
	for uri := range seen {
		found := false
		for _, w := range want {
			if w == uri {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("extra live resource %s", uri)
		}
	}
}

func TestContractReads(t *testing.T) {
	s, svc := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)

	ver := structuredMap(t, callTool(t, cs, "mail_version_get", map[string]any{}))
	if ver["protocols"] == nil {
		t.Fatalf("version=%v", ver)
	}
	caps := structuredMap(t, callTool(t, cs, "mail_capabilities_get", map[string]any{}))
	if caps["capabilities"] == nil {
		t.Fatalf("capabilities=%v", caps)
	}
	st := structuredMap(t, callTool(t, cs, "mail_status_get", map[string]any{}))
	if st["revisions"] == nil {
		t.Fatalf("status=%v", st)
	}
	schema := callTool(t, cs, "mail_schema_get", map[string]any{})
	raw, _ := json.Marshal(schema.StructuredContent)
	if !strings.Contains(string(raw), "labmail.dev/v1alpha1") {
		t.Fatalf("schema missing api version: %s", raw)
	}

	state := structuredMap(t, callTool(t, cs, "mail_state_get", map[string]any{}))
	if state["runtimeRevision"] == "" {
		t.Fatalf("state=%v", state)
	}

	id := insertMail(t, svc, "hello", "see https://app.lab.test/verify?token=abc1 Your code is 123456")
	list := structuredMap(t, callTool(t, cs, "mail_messages_list", map[string]any{"limit": 1}))
	if list["storeGeneration"] == nil {
		t.Fatalf("list=%v", list)
	}
	items, _ := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items=%v", list)
	}
	item := items[0].(map[string]any)
	if _, ok := item["text"]; ok {
		t.Fatal("list item must omit text")
	}
	if item["hasHTML"] == nil {
		t.Fatal("list item missing hasHTML")
	}

	got := structuredMap(t, callTool(t, cs, "mail_message_get", map[string]any{"id": id}))
	if got["text"] == nil || got["read"] != false {
		t.Fatalf("get default markRead=false: %v", got)
	}
	marked := structuredMap(t, callTool(t, cs, "mail_message_get", map[string]any{"id": id, "markRead": true}))
	if marked["read"] != true {
		t.Fatal("markRead=true")
	}
	rawMsg := structuredMap(t, callTool(t, cs, "mail_message_raw_get", map[string]any{"id": id}))
	if rawMsg["contentType"] != "message/rfc822" {
		t.Fatalf("raw=%v", rawMsg)
	}
	missing := callToolExpectError(t, cs, "mail_message_get", map[string]any{"id": "01AAAAAAAAAAAAAAAAAAAAAAAA"})
	if domainCode(t, missing) != "not_found" {
		t.Fatalf("missing message error=%v", missing)
	}

	stateRes, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "labmail://state"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stateRes.Contents) == 0 || !strings.Contains(stateRes.Contents[0].Text, "runtimeRevision") {
		t.Fatalf("resource state=%+v", stateRes)
	}
	msgRes, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "labmail://messages/" + id})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgRes.Contents) == 0 || !strings.Contains(msgRes.Contents[0].Text, `"id"`) {
		t.Fatalf("resource message=%+v", msgRes)
	}
}

func TestContractMutations(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)

	state := structuredMap(t, callTool(t, cs, "mail_state_get", map[string]any{}))
	rev, _ := state["runtimeRevision"].(string)
	args := map[string]any{
		"expectedRevision": rev,
		"reason":           "hide SIZE",
		"operations": []model.Operation{{
			Op:             model.OpReplaceHideExtensions,
			HideExtensions: []string{"SIZE"},
		}},
	}
	val := structuredMap(t, callTool(t, cs, "mail_state_validate", map[string]any{
		"operations": []model.Operation{{
			Op:             model.OpReplaceHideExtensions,
			HideExtensions: []string{"SIZE"},
		}},
	}))
	if val["candidateRevision"] == nil && val["previousRevision"] == nil {
		t.Fatalf("validate=%v", val)
	}
	plan := structuredMap(t, callTool(t, cs, "mail_change_plan", args))
	if plan["candidateRevision"] == rev {
		t.Fatal("plan did not change revision")
	}
	bad := callToolExpectError(t, cs, "mail_change_apply", map[string]any{
		"expectedRevision": "sha256:deadbeef",
		"operations": []model.Operation{{
			Op:             model.OpReplaceHideExtensions,
			HideExtensions: []string{"SIZE"},
		}},
	})
	if domainCode(t, bad) != "revision_conflict" {
		t.Fatalf("apply conflict=%v", bad)
	}
	apply := structuredMap(t, callTool(t, cs, "mail_change_apply", args))
	if apply["applied"] != true {
		t.Fatalf("apply=%v", apply)
	}
	exp := structuredMap(t, callTool(t, cs, "mail_state_export", map[string]any{"format": "yaml"}))
	body, _ := exp["body"].(string)
	if !strings.Contains(body, "apiVersion") {
		t.Fatalf("export missing body: %v", exp)
	}
	reset := structuredMap(t, callTool(t, cs, "mail_state_reset", map[string]any{"reason": "test"}))
	if reset["applied"] != true {
		t.Fatalf("reset=%v", reset)
	}
}

func TestWaitAndExtract(t *testing.T) {
	s, svc := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	insertMail(t, svc, "mcplab smoke", "Visit https://app.lab.test/ok Your code is 482193")
	wait := structuredMap(t, callTool(t, cs, "mail_messages_wait", map[string]any{
		"filter":  map[string]any{"subjectContains": "smoke"},
		"timeout": "1s",
	}))
	id, _ := wait["id"].(string)
	if id == "" {
		t.Fatalf("wait=%v", wait)
	}
	ex := structuredMap(t, callTool(t, cs, "mail_message_extract", map[string]any{"id": id}))
	urls, _ := ex["urls"].([]any)
	if len(urls) != 1 || urls[0] != "https://app.lab.test/ok" {
		t.Fatalf("extract=%v", ex)
	}
	timed := callToolExpectError(t, cs, "mail_messages_wait", map[string]any{
		"filter":  map[string]any{"subject": "never"},
		"timeout": "1ms",
	})
	if domainCode(t, timed) != "timeout" {
		t.Fatalf("wait timeout=%v", timed)
	}
}

func TestMarkReadDefaultFalse(t *testing.T) {
	s, svc := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	id := insertMail(t, svc, "unread", "body")
	got := structuredMap(t, callTool(t, cs, "mail_message_get", map[string]any{"id": id}))
	if got["read"] != false {
		t.Fatal("native get must default markRead=false")
	}
}

func TestHealthNotRegisteredAsTools(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	for tool, err := range cs.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(tool.Name, "health") || tool.Name == "health.live" || tool.Name == "health.ready" {
			t.Fatalf("health probe leaked as tool %q", tool.Name)
		}
	}
}

func callToolExpectError(t *testing.T, cs *sdk.ClientSession, name string, args any) error {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return err
	}
	if res != nil && res.IsError {
		raw, _ := json.Marshal(res.StructuredContent)
		return &toolDomainError{raw: raw, text: firstText(res)}
	}
	t.Fatalf("CallTool %s: want error", name)
	return nil
}

type toolDomainError struct {
	raw  []byte
	text string
}

func (e *toolDomainError) Error() string { return e.text }

func firstText(res *sdk.CallToolResult) string {
	if res == nil {
		return ""
	}
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			return tc.Text
		}
	}
	return "tool error"
}

func domainCode(t *testing.T, err error) string {
	t.Helper()
	var te *toolDomainError
	if errors.As(err, &te) && len(te.raw) > 0 {
		var payload struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(te.raw, &payload) == nil && payload.Code != "" {
			return payload.Code
		}
	}
	var werr *jsonrpc.Error
	if errors.As(err, &werr) && len(werr.Data) > 0 {
		var payload struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(werr.Data, &payload) == nil && payload.Code != "" {
			return payload.Code
		}
	}
	s := err.Error()
	for _, code := range []string{"not_found", "revision_conflict", "idempotency_conflict", "validation_failed", "timeout"} {
		if strings.Contains(s, code) {
			return code
		}
	}
	return s
}
