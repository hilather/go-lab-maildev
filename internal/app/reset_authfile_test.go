package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

const resetAuthToken = "0123456789abcdef0123456789abcdef"

// TestResetRefusesUnreadableBasicPassword: bearer_and_basic with a username
// still refuses a candidate whose passwordFile is missing or comment-only.
// The refusal is auth.Preflight (FromSpec), before the inbox wipe.
func TestResetRefusesUnreadableBasicPassword(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		assertResetRefusesBasicPassword(t, func(dir string) string {
			return filepath.Join(dir, "missing.pass")
		})
	})
	t.Run("comment_only", func(t *testing.T) {
		assertResetRefusesBasicPassword(t, func(dir string) string {
			path := filepath.Join(dir, "pass")
			if err := os.WriteFile(path, []byte("# keep\n\n  \n# out\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		})
	})
}

func assertResetRefusesBasicPassword(t *testing.T, passwordPath func(dir string) string) {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	pw := filepath.Join(dir, "boot.pass")
	if err := os.WriteFile(tok, []byte(resetAuthToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pw, []byte("lab-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	body := basicResetDoc("bearer_and_basic", tok, pw, "labmail.lab")
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := bootResetAuth(t, cfg)
	id := insertRaw(t, svc, "keep")
	rev := svc.Active().Revision

	bad := passwordPath(dir)
	next := basicResetDoc("bearer_and_basic", tok, bad, "labmail.lab")
	if err := os.WriteFile(cfg, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Reset(context.Background(), actor(), ResetIn{Reason: "rotate-password"})
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("reset err=%v want validation_failed", err)
	}
	if de.Message != "basic password is unavailable" {
		t.Fatalf("message=%q want auth preflight basic password is unavailable", de.Message)
	}
	named := false
	for _, fv := range de.FieldViolations {
		if fv.Path == "spec.management.auth.basic.passwordFile" && fv.Code == "unresolved_reference" && strings.Contains(fv.Message, bad) {
			named = true
		}
	}
	if !named {
		t.Fatalf("violations=%+v want unresolved passwordFile %s", de.FieldViolations, bad)
	}
	if got := svc.Active().Revision; got != rev {
		t.Fatalf("revision changed %s -> %s", rev, got)
	}
	if _, err := svc.Inbox().Get(id, false); err != nil {
		t.Fatalf("inbox changed after refused reset: %v", err)
	}
}

// TestResetBearerStalePasswordFileStillResets: bearer mode does not read
// basic.passwordFile. A candidate that leaves a stale password path still
// resets, wipes the inbox, and swaps the snapshot.
func TestResetBearerStalePasswordFileStillResets(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	pw := filepath.Join(dir, "boot.pass")
	if err := os.WriteFile(tok, []byte(resetAuthToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pw, []byte("lab-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	if err := os.WriteFile(cfg, []byte(basicResetDoc("bearer", tok, pw, "before.lab")), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := bootResetAuth(t, cfg)
	id := insertRaw(t, svc, "wipe-me")
	before := svc.Active().Revision

	missing := filepath.Join(dir, "missing.pass")
	if err := os.WriteFile(cfg, []byte(basicResetDoc("bearer", tok, missing, "after.lab")), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Reset(context.Background(), actor(), ResetIn{Reason: "bearer-stale-password"})
	if err != nil {
		t.Fatalf("reset err=%v want success", err)
	}
	if !res.Applied {
		t.Fatal("reset did not apply")
	}
	if svc.Active().Revision == before {
		t.Fatal("revision did not change")
	}
	spec := svc.Active().Canonical.Spec
	if spec.SMTP.Hostname != "after.lab" {
		t.Fatalf("hostname=%q", spec.SMTP.Hostname)
	}
	if spec.Management.Auth.Mode != model.MgmtAuthBearer {
		t.Fatalf("mode=%q", spec.Management.Auth.Mode)
	}
	if spec.Management.Auth.Basic.PasswordFile != missing {
		t.Fatalf("passwordFile=%q", spec.Management.Auth.Basic.PasswordFile)
	}
	if _, err := svc.Inbox().Get(id, false); err == nil {
		t.Fatal("inbox survived a successful reset")
	}
}

func bootResetAuth(t *testing.T, cfg string) *App {
	t.Helper()
	svc, err := Boot(context.Background(), Options{BootstrapPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	svc.OnAuthPreflight(auth.Preflight)
	return svc
}

func basicResetDoc(mode, tokenFile, passwordFile, hostname string) string {
	return "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  smtp:\n    hostname: " + hostname + "\n  management:\n    auth:\n      mode: " + mode + "\n      tokens:\n        - id: admin\n          secretFile: " + tokenFile + "\n          role: administrator\n      basic:\n        username: admin\n        passwordFile: " + passwordFile + "\n        tokenRef: admin\n"
}
