package auth

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

const testToken = "0123456789abcdef0123456789abcdef"

func writeSecret(t *testing.T, dir, name, value string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func testSpec(t *testing.T, mode string) (model.MgmtAuthSpec, string) {
	t.Helper()
	dir := t.TempDir()
	tok := writeSecret(t, dir, "token", testToken)
	pw := writeSecret(t, dir, "pass", "lab-web-pass")
	return model.MgmtAuthSpec{
		Mode: mode,
		Tokens: []model.TokenSpec{{
			ID:         "admin",
			SecretFile: tok,
			Role:       model.RoleAdministrator,
		}},
		Basic: model.BasicSpec{
			Username:     "admin",
			PasswordFile: pw,
			TokenRef:     "admin",
		},
	}, dir
}

func TestBearerAndBasicSamePrincipal(t *testing.T) {
	spec, _ := testSpec(t, model.MgmtAuthBearerAndBasic)
	v, err := FromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !v.BasicEnabled() {
		t.Fatal("basic should be enabled")
	}
	bearer, err := v.Authenticate(Request{Authorization: "Bearer " + testToken})
	if err != nil {
		t.Fatal(err)
	}
	basic, err := v.Authenticate(Request{
		Authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:lab-web-pass")),
		AllowBasic:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bearer.ID != "admin" || basic.ID != "admin" {
		t.Fatalf("ids bearer=%q basic=%q", bearer.ID, basic.ID)
	}
	if bearer.Role != model.RoleAdministrator || basic.Role != bearer.Role {
		t.Fatalf("roles %+v %+v", bearer, basic)
	}
	if !bearer.HasScope(model.ScopeMailAdmin) || !basic.HasScope(model.ScopeMailRead) {
		t.Fatalf("scopes %+v", bearer.Scopes)
	}
}

func TestMCPRejectsBasicEvenWhenEnabled(t *testing.T) {
	spec, _ := testSpec(t, model.MgmtAuthBearerAndBasic)
	v, err := FromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.Authenticate(Request{
		Authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:lab-web-pass")),
		AllowBasic:    false,
	})
	if err == nil {
		t.Fatal("MCP must reject Basic")
	}
}

func TestMissingAndBadSecrets(t *testing.T) {
	spec, _ := testSpec(t, model.MgmtAuthBearerAndBasic)
	v, err := FromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Authenticate(Request{}); err == nil {
		t.Fatal("missing auth accepted")
	}
	if _, err := v.Authenticate(Request{Authorization: "Bearer wrong-token-value-not-32b-xxxxx"}); err == nil {
		t.Fatal("bad bearer accepted")
	}
	if _, err := v.Authenticate(Request{
		Authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:nope")),
		AllowBasic:    true,
	}); err == nil {
		t.Fatal("bad basic accepted")
	}
	if _, err := v.Authenticate(Request{
		Authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("other:lab-web-pass")),
		AllowBasic:    true,
	}); err == nil {
		t.Fatal("wrong user accepted")
	}
}

func TestShortTokenRejected(t *testing.T) {
	dir := t.TempDir()
	tok := writeSecret(t, dir, "token", "tooshort")
	_, err := FromSpec(model.MgmtAuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID: "admin", SecretFile: tok, Role: model.RoleAdministrator,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "256") && !strings.Contains(err.Error(), "32") {
		t.Fatalf("short token: %v", err)
	}
}

func TestLoopbackUnauth(t *testing.T) {
	spec, _ := testSpec(t, model.MgmtAuthDevLoopbackUnauth)
	v, err := FromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.Authenticate(Request{RemoteAddr: "127.0.0.1:9"})
	if err != nil || p.Class != ClassLoopback {
		t.Fatalf("loopback: %+v %v", p, err)
	}
	if _, err := v.Authenticate(Request{RemoteAddr: "192.168.1.9:9"}); err == nil {
		t.Fatal("non-loopback unauth accepted")
	}
	p, err = v.Authenticate(Request{Authorization: "Bearer " + testToken, RemoteAddr: "192.168.1.9:9"})
	if err != nil || p.ID != "admin" {
		t.Fatalf("bearer on lan: %+v %v", p, err)
	}
}

func TestBearerModeIgnoresBasic(t *testing.T) {
	spec, _ := testSpec(t, model.MgmtAuthBearer)
	v, err := FromSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if v.BasicEnabled() {
		t.Fatal("basic enabled in bearer mode")
	}
	if _, err := v.Authenticate(Request{
		Authorization: "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:lab-web-pass")),
		AllowBasic:    true,
	}); err == nil {
		t.Fatal("basic accepted in bearer mode")
	}
}

func TestMailAdminSatisfiesRead(t *testing.T) {
	p := Principal{Scopes: []string{model.ScopeMailAdmin}}
	if err := Authorize(p, []string{model.ScopeMailRead, model.ScopeMailWrite, model.ScopeMailAuditRead}); err != nil {
		t.Fatal(err)
	}
	op := Principal{Scopes: DefaultScopes(model.RoleOperator)}
	if err := Authorize(op, []string{model.ScopeMailAdmin}); err == nil {
		t.Fatal("operator must not reset")
	}
}

func TestWWWAuthenticate(t *testing.T) {
	got := WWWAuthenticate(true)
	if len(got) != 2 || !strings.Contains(got[0], "Bearer") || !strings.Contains(got[1], "Basic") {
		t.Fatalf("%v", got)
	}
	if len(WWWAuthenticate(false)) != 1 {
		t.Fatal("bearer-only challenge")
	}
}
