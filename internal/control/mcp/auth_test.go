package mcp

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestBasicRejected(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerAuthorization:   "Basic YWRtaW46c2VjcmV0",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate")
	}
}

func TestBearerStubAccepted(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerAuthorization:   "Bearer stub-token",
	}, "127.0.0.1:1")
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("bearer rejected: %s", rec.Body.String())
	}
}

func TestMCPAuthVerifierBearerOnly(t *testing.T) {
	s, _ := newTestServer(t)
	v, token := testVerifier(t)
	s.cfg.Auth = v
	hdr := map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
	}
	hdr[headerAuthorization] = "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:lab-web-pass"))
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), hdr, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")

	hdr[headerAuthorization] = "Bearer " + token
	rec = doRaw(t, s.Handler(), rpcCall(1, "tools/call", map[string]any{
		"name": "mail_messages_list", "arguments": map[string]any{"limit": 1},
	}), hdr, "127.0.0.1:1")
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("valid bearer tool rejected: %s", rec.Body.String())
	}

	delete(hdr, headerAuthorization)
	rec = doRaw(t, s.Handler(), rpcCall(1, "ping", nil), hdr, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func TestMCPScopedToolsUnderVerifier(t *testing.T) {
	s, _ := newTestServer(t)
	v, adminTok, viewTok := testVerifierWithViewer(t)
	s.cfg.Auth = v
	ts := startHTTP(t, s)

	admin := connectClientAuth(t, ts, adminTok)
	if res := callTool(t, admin, "mail_messages_list", map[string]any{"limit": 1}); res.IsError {
		t.Fatalf("admin list: %+v", res)
	}

	viewer := connectClientAuth(t, ts, viewTok)
	if res := callTool(t, viewer, "mail_messages_list", map[string]any{"limit": 1}); res.IsError {
		t.Fatalf("viewer list: %+v", res)
	}
	denied := callTool(t, viewer, "mail_state_reset", map[string]any{})
	if !denied.IsError {
		t.Fatal("viewer reset must be forbidden")
	}
}

func TestReloadAuthIdentityChangeCancelsLongRequests(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(dir, "pass")
	if err := os.WriteFile(pw, []byte("lab-web-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer_and_basic\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n      basic:\n        username: admin\n        passwordFile: " + pw + "\n        tokenRef: admin\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: v, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	ctx, cancel := s.trackLongRequest(context.Background())
	defer cancel()

	snap := svc.Active()
	if len(snap.Canonical.Spec.Management.Auth.Tokens) != 1 {
		t.Fatal("expected one token")
	}
	snap.Canonical.Spec.Management.Auth.Tokens[0].Role = model.RoleViewer
	snap.Canonical.Spec.Management.Auth.Tokens[0].Scopes = nil
	s.reloadAuth()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("MCP long request stayed open after auth identity change")
	}
}
