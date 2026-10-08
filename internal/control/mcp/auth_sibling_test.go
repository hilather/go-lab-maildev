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

// TestResetRemovesTokenWhileSiblingSecretUnreadable is QA's reset case: the
// candidate drops token X and points token Y's secretFile at a file that
// cannot be read. Reset is validation_failed. X keeps its previous role, Y
// stays, and the inbox is not wiped.
func TestResetRemovesTokenWhileSiblingSecretUnreadable(t *testing.T) {
	dir := t.TempDir()
	tokX := filepath.Join(dir, "x.token")
	tokY := filepath.Join(dir, "y.token")
	if err := os.WriteFile(tokX, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokY, []byte(testViewerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: x\n          secretFile: " + tokX + "\n          role: administrator\n        - id: y\n          secretFile: " + tokY + "\n          role: viewer\n"
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
	s, err := New(Config{Service: svc, Auth: v, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	raw := []byte("Subject: keep\r\n\r\nbody\r\n")
	ins, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), &model.Message{Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	rev := svc.Active().Revision

	missing := filepath.Join(dir, "missing.token")
	next := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: y\n          secretFile: " + missing + "\n          role: viewer\n"
	if err := os.WriteFile(cfgPath, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "drop-x"})
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("reset err=%v want validation_failed", err)
	}
	named := false
	for _, fv := range de.FieldViolations {
		if strings.Contains(fv.Message, missing) && fv.Code == "unresolved_reference" {
			named = true
		}
	}
	if !named {
		t.Fatalf("violations=%+v want unresolved secret %s", de.FieldViolations, missing)
	}
	if got := svc.Active().Revision; got != rev {
		t.Fatalf("revision changed %s -> %s", rev, got)
	}
	ids := map[string]string{}
	for _, tok := range svc.Active().Canonical.Spec.Management.Auth.Tokens {
		ids[tok.ID] = tok.Role
	}
	if ids["x"] != model.RoleAdministrator || ids["y"] != model.RoleViewer || len(ids) != 2 {
		t.Fatalf("snapshot tokens=%v want x administrator and y viewer", ids)
	}
	gotX, err := v.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatalf("token X rejected after refused reset: %v", err)
	}
	if gotX.Role != model.RoleAdministrator || gotX.ID != "x" {
		t.Fatalf("token X id=%s role=%s", gotX.ID, gotX.Role)
	}
	gotY, err := v.AuthenticateBearer(testViewerToken)
	if err != nil {
		t.Fatalf("token Y rejected after refused reset: %v", err)
	}
	if gotY.Role != model.RoleViewer || gotY.ID != "y" {
		t.Fatalf("token Y id=%s role=%s", gotY.ID, gotY.Role)
	}
	if _, err := svc.Inbox().Get(ins.ID, false); err != nil {
		t.Fatalf("inbox changed after refused reset: %v", err)
	}
}
