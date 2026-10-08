package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// TestStdioActorLosesAdminAfterDemotion is the mcp-stdio counterpart of the
// REST session clear: after bootstrap reload demotes the token, the process
// must not keep authorizing mail.admin with the snapshotted FixedActor.
func TestStdioActorLosesAdminAfterDemotion(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatal(err)
	}
	fixed := app.Actor{
		ID: p.ID, Class: p.Class, Role: p.Role,
		Scopes: append([]string(nil), p.Scopes...), Transport: "mcp",
	}
	s, err := New(Config{Service: svc, Auth: v, FixedActor: &fixed, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.authorizeTool(s.actorFrom(context.Background()), "mail_state_reset"); err != nil {
		t.Fatalf("precondition: stdio admin must be allowed to reset: %v", err)
	}

	demoted := strings.Replace(body, "role: administrator", "role: viewer", 1)
	if err := os.WriteFile(cfgPath, []byte(demoted), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "demote"}); err != nil {
		t.Fatal(err)
	}
	now, err := s.cfg.Auth.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatal(err)
	}
	if now.Role != model.RoleViewer {
		t.Fatalf("verifier did not demote token: role=%s scopes=%v", now.Role, now.Scopes)
	}
	if err := auth.AuthorizeScopes(now.Scopes, []string{model.ScopeMailAdmin}); err == nil {
		t.Fatal("demoted token still has mail.admin on the verifier")
	}
	if err := s.authorizeTool(s.actorFrom(context.Background()), "mail_state_reset"); err == nil {
		t.Fatal("stdio FixedActor still authorizes mail_state_reset after the token was demoted")
	}
}

// TestStdioSecretDemotionKeepsMailRead re-authenticates the stored startup
// secret. A viewer demotion loses mail.admin and keeps mail.read.
func TestStdioSecretDemotionKeepsMailRead(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatal(err)
	}
	fixed := app.Actor{
		ID: p.ID, Class: p.Class, Role: p.Role,
		Scopes: append([]string(nil), p.Scopes...), Transport: "mcp",
	}
	s, err := New(Config{
		Service: svc, Auth: v, FixedActor: &fixed,
		StdioSecret: testBearerToken, RatePerSec: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	demoted := strings.Replace(body, "role: administrator", "role: viewer", 1)
	if err := os.WriteFile(cfgPath, []byte(demoted), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "demote"}); err != nil {
		t.Fatal(err)
	}
	actor := s.actorFrom(context.Background())
	if actor.Role != model.RoleViewer {
		t.Fatalf("stdio actor role=%s want viewer", actor.Role)
	}
	if err := s.authorizeTool(actor, "mail_state_reset"); err == nil {
		t.Fatal("demoted stdio actor still authorizes mail_state_reset")
	}
	if err := s.authorizeTool(actor, "mail_version_get"); err != nil {
		t.Fatalf("demoted stdio actor lost mail.read: %v", err)
	}
}

// TestStdioFixedActorRestoredAfterTokenReadded removes the token entry from a
// spec that still compiles, then puts the same entry and secret file back.
func TestStdioFixedActorRestoredAfterTokenReadded(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatal(err)
	}
	fixed := app.Actor{
		ID: p.ID, Class: p.Class, Role: p.Role,
		Scopes: append([]string(nil), p.Scopes...), Transport: "mcp",
	}
	s, err := New(Config{
		Service: svc, Auth: v, FixedActor: &fixed,
		StdioSecret: testBearerToken, RatePerSec: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.authorizeTool(s.actorFrom(context.Background()), "mail_state_reset"); err != nil {
		t.Fatalf("precondition: %v", err)
	}

	removed := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n"
	if err := os.WriteFile(cfgPath, []byte(removed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "remove"}); err != nil {
		t.Fatal(err)
	}
	if err := s.authorizeTool(s.actorFrom(context.Background()), "mail_state_reset"); err == nil {
		t.Fatal("removed token still authorizes mail_state_reset")
	}

	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "restore"}); err != nil {
		t.Fatal(err)
	}
	actor := s.actorFrom(context.Background())
	if actor.Role != model.RoleAdministrator {
		t.Fatalf("restored role=%s scopes=%v", actor.Role, actor.Scopes)
	}
	if err := s.authorizeTool(actor, "mail_state_reset"); err != nil {
		t.Fatalf("restored administrator denied mail_state_reset: %v", err)
	}
}
