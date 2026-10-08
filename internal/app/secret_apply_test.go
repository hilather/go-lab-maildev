package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// TestPlanApplySMTPOnlyAfterManagementSecretDeleted: an SMTP-only plan and
// apply must succeed after the live spec's management secret files are
// removed. Absent files do not fail plan or apply. state:validate reads them.
func TestPlanApplySMTPOnlyAfterManagementSecretDeleted(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	pw := filepath.Join(dir, "pass")
	if err := os.WriteFile(tok, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pw, []byte("lab-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer_and_basic\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n      basic:\n        username: admin\n        passwordFile: " + pw + "\n        tokenRef: admin\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := Boot(context.Background(), Options{BootstrapPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	if err := os.Remove(tok); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(pw); err != nil {
		t.Fatal(err)
	}

	boot := svc.Active()
	in := ChangeIn{
		ExpectedRevision: boot.Revision,
		IdempotencyKey:   "smtp-absent-secret",
		Reason:           "hide SIZE",
		Operations:       []model.Operation{hideSIZE()},
	}
	plan, err := svc.Plan(context.Background(), actor(), in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.CandidateRevision == boot.Revision {
		t.Fatal("plan did not compile a new revision")
	}
	res, err := svc.Apply(context.Background(), actor(), in)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied || res.RuntimeRevision != plan.CandidateRevision {
		t.Fatalf("applied=%v rev=%s plan=%s", res.Applied, res.RuntimeRevision, plan.CandidateRevision)
	}
	got := svc.Active().Canonical.Spec.SMTP.HideExtensions
	if len(got) != 1 || got[0] != "SIZE" {
		t.Fatalf("hide=%v", got)
	}
	auth := svc.Active().Canonical.Spec.Management.Auth
	if len(auth.Tokens) != 1 || auth.Tokens[0].SecretFile != tok {
		t.Fatalf("secretFile=%v want %s", auth.Tokens, tok)
	}
	if auth.Basic.PasswordFile != pw {
		t.Fatalf("passwordFile=%q want %s", auth.Basic.PasswordFile, pw)
	}

	_, err = svc.Validate(context.Background(), actor(), ValidateIn{
		Operations: []model.Operation{hideSIZE()},
	})
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("validate err=%v want validation_failed", err)
	}
	sawTok, sawPw := false, false
	for _, fv := range de.FieldViolations {
		if strings.Contains(fv.Message, tok) {
			sawTok = true
		}
		if strings.Contains(fv.Message, pw) {
			sawPw = true
		}
	}
	if !sawTok || !sawPw {
		t.Fatalf("violations=%+v want both absent auth files", de.FieldViolations)
	}
}
