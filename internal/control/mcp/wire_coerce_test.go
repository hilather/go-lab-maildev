package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func callToolRaw(t *testing.T, s *Server, name, args string) (body string, code int) {
	t.Helper()
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":%q,"io.modelcontextprotocol/clientInfo":{"name":"labmail-repro","version":"dev"},"io.modelcontextprotocol/clientCapabilities":{}},"name":%q,"arguments":%s}}`, ProtocolVersion, name, args)
	rec := doRaw(t, s.Handler(), payload, map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerMethod:          "tools/call",
		headerName:            name,
	}, "127.0.0.1:9")
	return rec.Body.String(), rec.Code
}

// TestMCPRejectsBareDurationLikeREST: REST rejects greetingDelay: 30.
// MCP must not install that number as 30ns.
func TestMCPRejectsBareDurationLikeREST(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	args := `{"expectedRevision":"` + rev + `","reason":"repro","operations":[{"op":"replaceSMTPBehavior","behavior":{"greetingDelay":30,"commandDelay":0,"dropOnConnect":false,"closeAfterVerb":"","replies":{"greeting":"","helo":"","ehlo":"","mail":"","rcpt":"","data":"","dataEnd":"","rset":"","noop":"","vrfy":"","auth":"","starttls":"","unknown":""}}}]}`
	raw, code := callToolRaw(t, s, "mail_change_apply", args)
	if got := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay; got != 0 {
		t.Fatalf("MCP applied bare greetingDelay 30 as %s; body=%s", got, raw)
	}
	if !strings.Contains(raw, "validation_failed") && !strings.Contains(raw, "bare number") {
		t.Fatalf("bare greetingDelay was not rejected as a duration string: status=%d body=%s", code, raw)
	}
}

// TestMCPAcceptsDurationStringLikeREST: REST applies greetingDelay "30s".
// MCP must accept the same string.
func TestMCPAcceptsDurationStringLikeREST(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	args := `{"expectedRevision":"` + rev + `","reason":"repro","operations":[{"op":"replaceSMTPBehavior","behavior":{"greetingDelay":"30s","commandDelay":"0s","dropOnConnect":false,"closeAfterVerb":"","replies":{"greeting":"","helo":"","ehlo":"","mail":"","rcpt":"","data":"","dataEnd":"","rset":"","noop":"","vrfy":"","auth":"","starttls":"","unknown":""}}}]}`
	raw, code := callToolRaw(t, s, "mail_change_apply", args)
	got := svc.Active().Canonical.Spec.SMTP.Behavior.GreetingDelay
	if got != 30*time.Second {
		t.Fatalf("MCP greetingDelay \"30s\" applied as %s; status=%d body=%s", got, code, raw)
	}
}

// TestMCPRejectsBareByteSizeLikeREST: maxBytes: 1 must not shrink the inbox
// to one byte. REST rejects the bare number.
func TestMCPRejectsBareByteSizeLikeREST(t *testing.T) {
	s, svc := newTestServer(t)
	before := svc.Active().Canonical.Spec.Store.MaxBytes
	rev := string(svc.Active().Revision)
	args := `{"expectedRevision":"` + rev + `","reason":"repro","force":true,"operations":[{"op":"replaceStoreCaps","store":{"maxMessages":1000,"maxBytes":1,"fullPolicy":"evict_oldest"}}]}`
	raw, code := callToolRaw(t, s, "mail_change_apply", args)
	if got := svc.Active().Canonical.Spec.Store.MaxBytes; got != before {
		t.Fatalf("MCP applied bare maxBytes 1 as %d (was %d); body=%s", got, before, raw)
	}
	if !strings.Contains(raw, "validation_failed") && !strings.Contains(raw, "byte size") {
		t.Fatalf("bare maxBytes was not rejected: status=%d body=%s", code, raw)
	}
}

// TestMCPChangeApplySchemaListsStringAndInteger: tools/list must advertise
// both string and integer for greetingDelay and maxBytes so a bare number
// reaches the coercer and a duration string is not rejected as an integer.
func TestMCPChangeApplySchemaListsStringAndInteger(t *testing.T) {
	s, _ := newTestServer(t)
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":%q,"io.modelcontextprotocol/clientInfo":{"name":"labmail-repro","version":"dev"},"io.modelcontextprotocol/clientCapabilities":{}}}}`, ProtocolVersion)
	rec := doRaw(t, s.Handler(), payload, map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerMethod:          "tools/list",
	}, "127.0.0.1:9")
	raw := rec.Body.String()
	schema := toolInputSchema(t, raw, "mail_change_apply")
	gd, ok := findSchemaProp(schema, "greetingDelay")
	if !ok {
		t.Fatalf("greetingDelay missing from mail_change_apply schema: %s", raw)
	}
	mb, ok := findSchemaProp(schema, "maxBytes")
	if !ok {
		t.Fatalf("maxBytes missing from mail_change_apply schema: %s", raw)
	}
	assertStringAndInteger(t, gd, "greetingDelay")
	assertStringAndInteger(t, mb, "maxBytes")
}

func toolInputSchema(t *testing.T, raw, name string) map[string]any {
	t.Helper()
	body := raw
	if i := strings.Index(body, "data:"); i >= 0 {
		line := body[i+len("data:"):]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		body = strings.TrimSpace(line)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("tools/list json: %v body=%s", err, raw)
	}
	result, _ := env["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	for _, item := range tools {
		tool, _ := item.(map[string]any)
		if tool["name"] != name {
			continue
		}
		schema, _ := tool["inputSchema"].(map[string]any)
		if schema == nil {
			t.Fatalf("%s inputSchema missing: %s", name, raw)
		}
		return schema
	}
	t.Fatalf("%s missing from tools/list: %s", name, raw)
	return nil
}

func findSchemaProp(v any, name string) (map[string]any, bool) {
	switch x := v.(type) {
	case map[string]any:
		if props, ok := x["properties"].(map[string]any); ok {
			if p, ok := props[name].(map[string]any); ok {
				return p, true
			}
		}
		for _, child := range x {
			if found, ok := findSchemaProp(child, name); ok {
				return found, true
			}
		}
	case []any:
		for _, child := range x {
			if found, ok := findSchemaProp(child, name); ok {
				return found, true
			}
		}
	}
	return nil, false
}

func assertStringAndInteger(t *testing.T, prop map[string]any, name string) {
	t.Helper()
	types := schemaTypes(prop["type"])
	hasString, hasInteger := false, false
	for _, typ := range types {
		switch typ {
		case "string":
			hasString = true
		case "integer":
			hasInteger = true
		}
	}
	if !hasString || !hasInteger {
		t.Fatalf("%s schema types=%v want string and integer; prop=%v", name, types, prop)
	}
	if len(types) == 1 && types[0] == "integer" {
		t.Fatalf("%s schema is integer-only", name)
	}
}

func schemaTypes(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// TestMCPValidateStateOnlyNoOperations: omitted operations with a candidate
// state document still validates. Empty raw operations must not be decoded.
func TestMCPValidateStateOnlyNoOperations(t *testing.T) {
	s, svc := newTestServer(t)
	canon, err := marshalAPI(svc.Active().Canonical)
	if err != nil {
		t.Fatal(err)
	}
	raw, code := callToolRaw(t, s, "mail_state_validate", `{"state":`+string(canon)+`}`)
	if !strings.Contains(raw, "candidateRevision") && !strings.Contains(raw, "previousRevision") {
		t.Fatalf("state-only validate failed: status=%d body=%s", code, raw)
	}
}

// TestMCPApplyOmittedOperations: plan and apply with no operations field
// behave as an empty operation list.
func TestMCPApplyOmittedOperations(t *testing.T) {
	s, svc := newTestServer(t)
	rev := string(svc.Active().Revision)
	before := svc.Active().Canonical.Spec.Store.MaxBytes
	for _, tool := range []string{"mail_change_plan", "mail_change_apply"} {
		raw, code := callToolRaw(t, s, tool, `{"expectedRevision":"`+rev+`","reason":"repro"}`)
		if strings.Contains(raw, "validation_failed") {
			t.Fatalf("%s omitted operations rejected: status=%d body=%s", tool, code, raw)
		}
	}
	raw, code := callToolRaw(t, s, "mail_change_apply", `{"expectedRevision":"`+rev+`","reason":"repro","operations":null}`)
	if strings.Contains(raw, "validation_failed") {
		t.Fatalf("null operations rejected: status=%d body=%s", code, raw)
	}
	if got := svc.Active().Canonical.Spec.Store.MaxBytes; got != before {
		t.Fatalf("omitted operations changed maxBytes to %d; body=%s", got, raw)
	}
	if !strings.Contains(raw, `"applied":true`) {
		t.Fatalf("omitted operations apply: status=%d body=%s", code, raw)
	}
}
