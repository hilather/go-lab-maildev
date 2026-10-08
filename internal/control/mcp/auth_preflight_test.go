package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// TestResetUnreadableSecretKeepsSnapshotAndVerifier refuses a reset whose
// candidate demotes the admin token and points secretFile at a file the
// verifier cannot read. The previous snapshot, bearer, and stdio actor stay.
func TestResetUnreadableSecretKeepsSnapshotAndVerifier(t *testing.T) {
	t.Run("missing secret file", func(t *testing.T) {
		assertResetUnreadableSecret(t, func(dir string) string {
			return filepath.Join(dir, "missing.token")
		})
	})
	t.Run("mode 000 secret file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can read a mode 000 file")
		}
		assertResetUnreadableSecret(t, func(dir string) string {
			path := filepath.Join(dir, "unreadable.token")
			if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatal(err)
			}
			return path
		})
	})
}

func assertResetUnreadableSecret(t *testing.T, secretPath func(dir string) string) {
	t.Helper()
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
		t.Fatalf("precondition: stdio admin must be allowed to reset: %v", err)
	}
	raw := []byte("Subject: keep\r\n\r\nbody\r\n")
	ins, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), &model.Message{Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	rev := svc.Active().Revision

	missing := secretPath(dir)
	next := strings.Replace(body, "role: administrator", "role: viewer", 1)
	next = strings.Replace(next, tok, missing, 1)
	if err := os.WriteFile(cfgPath, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "rotate"})
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("reset err=%v want validation_failed", err)
	}
	named := false
	for _, fv := range de.FieldViolations {
		if fv.Path == "spec.management.auth.tokens[0].secretFile" && strings.Contains(fv.Message, missing) {
			named = true
		}
	}
	if !named {
		t.Fatalf("violations=%+v want secretFile %s", de.FieldViolations, missing)
	}
	if got := svc.Active().Revision; got != rev {
		t.Fatalf("revision changed %s -> %s", rev, got)
	}
	now, err := v.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatalf("old bearer rejected: %v", err)
	}
	if now.Role != model.RoleAdministrator {
		t.Fatalf("verifier role=%s want administrator", now.Role)
	}
	if err := s.authorizeTool(s.actorFrom(context.Background()), "mail_state_reset"); err != nil {
		t.Fatalf("stdio actor lost mail_state_reset: %v", err)
	}
	if _, err := svc.Inbox().Get(ins.ID, false); err != nil {
		t.Fatalf("inbox changed after refused reset: %v", err)
	}
}
