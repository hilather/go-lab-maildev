package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// TestLabOverlayExample loads examples/labmail.yaml (the mcp-integration-lab
// bootstrap) and checks the swap knobs the lab PR must not regress.
func TestLabOverlayExample(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "labmail.yaml")
	st, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Spec.Management.MCP.AllowLegacyClients {
		t.Fatal("lab overlay must set allowLegacyClients: true (D17)")
	}
	if st.Spec.Management.Auth.Mode != model.MgmtAuthBearerAndBasic {
		t.Fatalf("auth.mode=%q", st.Spec.Management.Auth.Mode)
	}
	if st.Spec.Management.Auth.Basic.TokenRef != "admin" {
		t.Fatalf("basic.tokenRef=%q", st.Spec.Management.Auth.Basic.TokenRef)
	}
	if st.Spec.Management.Auth.Basic.Username != "admin" {
		t.Fatalf("basic.username=%q", st.Spec.Management.Auth.Basic.Username)
	}
	if st.Spec.Management.Auth.Basic.PasswordFile != "/run/secrets/maildev-web-password" {
		t.Fatalf("basic.passwordFile=%q", st.Spec.Management.Auth.Basic.PasswordFile)
	}
	if st.Spec.SMTP.Auth.Mode != model.SMTPAuthNone {
		t.Fatalf("smtp.auth.mode=%q want none (smoke SendMail with no AUTH)", st.Spec.SMTP.Auth.Mode)
	}
	if !st.Spec.Listeners.Management.CompatEnabled {
		t.Fatal("compatEnabled must stay true for GET /email")
	}
	if !st.Spec.UI.Enabled {
		t.Fatal("ui.enabled must stay true (Q2: inbox UI required for GA)")
	}
	if len(st.Spec.Management.OriginAllowlist) != 0 {
		t.Fatalf("lab overlay must keep originAllowlist empty, got %q", st.Spec.Management.OriginAllowlist)
	}
	if st.Spec.Listeners.SMTP.Address != ":1025" || st.Spec.Listeners.Management.Address != ":1080" {
		t.Fatalf("listeners smtp=%q mgmt=%q", st.Spec.Listeners.SMTP.Address, st.Spec.Listeners.Management.Address)
	}
	found := false
	for _, tok := range st.Spec.Management.Auth.Tokens {
		if tok.ID == "admin" && tok.SecretFile == "/run/secrets/labmail-token" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing admin token at /run/secrets/labmail-token")
	}
}

func TestLabMCPJungleExamples(t *testing.T) {
	root := filepath.Join(repoRoot(t), "examples", "mcpjungle")
	raw, err := os.ReadFile(filepath.Join(root, "servers", "labmail.json"))
	if err != nil {
		t.Fatal(err)
	}
	var server struct {
		Name        string `json:"name"`
		Transport   string `json:"transport"`
		URL         string `json:"url"`
		BearerToken string `json:"bearer_token"`
	}
	if err := json.Unmarshal(raw, &server); err != nil {
		t.Fatal(err)
	}
	if server.Name != "labmail" {
		t.Fatalf("name=%q (filename must match name; AGENTS.md rule 8)", server.Name)
	}
	if server.Transport != "streamable_http" {
		t.Fatalf("transport=%q", server.Transport)
	}
	if server.URL != "http://maildev:1080/mcp" {
		t.Fatalf("url=%q", server.URL)
	}
	if server.BearerToken != "${LABMAIL_TOKEN}" {
		t.Fatalf("bearer_token=%q", server.BearerToken)
	}

	grow, err := os.ReadFile(filepath.Join(root, "groups", "integration.json"))
	if err != nil {
		t.Fatal(err)
	}
	var group struct {
		Name            string   `json:"name"`
		IncludedServers []string `json:"included_servers"`
	}
	if err := json.Unmarshal(grow, &group); err != nil {
		t.Fatal(err)
	}
	if group.Name != "integration" {
		t.Fatalf("group name=%q", group.Name)
	}
	want := []string{"labdns", "labldap", "labtacacs", "labinfo", "labmail"}
	if !slices.Equal(group.IncludedServers, want) {
		t.Fatalf("included_servers=%v want %v (append labmail; do not replace)", group.IncludedServers, want)
	}
}

func TestLabinfoMaildevSnippetKeepsCatalogID(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "labinfo", "services-maildev.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "id: maildev") {
		t.Fatal("Q1/D15: catalog id must stay maildev")
	}
	if strings.Contains(text, "id: labmail") {
		t.Fatal("do not rename catalog id in the swap overlay")
	}
	if !strings.Contains(text, "Mail sink (LabMail, receive-only)") {
		t.Fatal("labinfo name must become Mail sink (LabMail, receive-only)")
	}
	if !strings.Contains(text, "/v1") || !strings.Contains(text, "/mcp") {
		t.Fatal("snippet must add native /v1 and MCP URLs")
	}
	if !strings.Contains(text, "labmail-token") {
		t.Fatal("snippet must add bearer credential file")
	}
	if !strings.Contains(text, "user 'admin'") {
		t.Fatal("Basic user is frozen at admin; do not interpolate MAILDEV_WEB_USER as the live username")
	}
}
