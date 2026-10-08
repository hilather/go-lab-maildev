package compiler

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestValidateMissingSecretFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.token")
	doc := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + missing + "\n          role: administrator\n"
	st, err := config.Load([]byte(doc))
	if err != nil {
		t.Fatalf("Load of an absent secretFile must still succeed: %v", err)
	}
	_, err = Compile(context.Background(), st, CompileOpts{})
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("compile err=%v want validation_failed", err)
	}
	if !authFileViolation(de, "spec.management.auth.tokens[0].secretFile", "unresolved_reference", missing) {
		t.Fatalf("violations=%+v want unresolved secretFile %s", de.FieldViolations, missing)
	}
}

func TestValidateMissingPasswordFile(t *testing.T) {
	dir := t.TempDir()
	tok := writeTokenFile(t, dir)
	missing := filepath.Join(dir, "missing.pass")
	st, err := config.Load([]byte(basicManagementDoc(model.MgmtAuthBearerAndBasic, tok, missing)))
	if err != nil {
		t.Fatalf("Load of an absent passwordFile must still succeed: %v", err)
	}
	_, err = Compile(context.Background(), st, CompileOpts{})
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("compile err=%v want validation_failed", err)
	}
	if !authFileViolation(de, "spec.management.auth.basic.passwordFile", "unresolved_reference", missing) {
		t.Fatalf("violations=%+v want unresolved passwordFile %s", de.FieldViolations, missing)
	}
}

// TestCompileBearerSkipsMissingPasswordFile: FromSpec reads basic.passwordFile
// only for bearer_and_basic with a username. A bearer document with a complete
// basic block and a missing password file must still compile.
func TestCompileBearerSkipsMissingPasswordFile(t *testing.T) {
	assertCompileSkipsMissingPasswordFile(t, model.MgmtAuthBearer)
}

// TestCompileDevLoopbackSkipsMissingPasswordFile is the same skip for
// dev-loopback-unauth.
func TestCompileDevLoopbackSkipsMissingPasswordFile(t *testing.T) {
	assertCompileSkipsMissingPasswordFile(t, model.MgmtAuthDevLoopbackUnauth)
}

func assertCompileSkipsMissingPasswordFile(t *testing.T, mode string) {
	t.Helper()
	dir := t.TempDir()
	tok := writeTokenFile(t, dir)
	missing := filepath.Join(dir, "missing.pass")
	st, err := config.Load([]byte(basicManagementDoc(mode, tok, missing)))
	if err != nil {
		t.Fatalf("Load of an absent passwordFile must still succeed: %v", err)
	}
	if _, err := Compile(context.Background(), st, CompileOpts{}); err != nil {
		t.Fatalf("compile err=%v want success for mode %s", err, mode)
	}
}

// TestCompileBlankPasswordFile: a readable password file with no usable line
// is the same unresolved_reference as a missing file. FromSpec's readSecretFile
// returns os.ErrInvalid for empty and comment-only files.
func TestCompileBlankPasswordFile(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "comment_only", body: "# keep\n\n  \n# out\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tok := writeTokenFile(t, dir)
			pw := filepath.Join(dir, "pass")
			if err := os.WriteFile(pw, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			st, err := config.Load([]byte(basicManagementDoc(model.MgmtAuthBearerAndBasic, tok, pw)))
			if err != nil {
				t.Fatalf("Load of a blank passwordFile must still succeed: %v", err)
			}
			_, err = Compile(context.Background(), st, CompileOpts{})
			de, ok := domainerr.As(err)
			if !ok || de.Code != domainerr.CodeValidationFailed {
				t.Fatalf("compile err=%v want validation_failed", err)
			}
			if !authFileViolation(de, "spec.management.auth.basic.passwordFile", "unresolved_reference", pw) {
				t.Fatalf("violations=%+v want unresolved passwordFile %s", de.FieldViolations, pw)
			}
		})
	}
}

func writeTokenFile(t *testing.T, dir string) string {
	t.Helper()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return tok
}

func basicManagementDoc(mode, tokenFile, passwordFile string) string {
	return "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: " + mode + "\n      tokens:\n        - id: admin\n          secretFile: " + tokenFile + "\n          role: administrator\n      basic:\n        username: admin\n        passwordFile: " + passwordFile + "\n        tokenRef: admin\n"
}

func authFileViolation(de *domainerr.Error, path, code, needle string) bool {
	if de == nil {
		return false
	}
	for _, fv := range de.FieldViolations {
		if fv.Path == path && fv.Code == code && strings.Contains(fv.Message, needle) {
			return true
		}
	}
	return false
}

func TestCompileNilState(t *testing.T) {
	_, err := Compile(context.Background(), nil, CompileOpts{})
	if err == nil {
		t.Fatal("nil state compiled")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("err=%v, want validation_failed", err)
	}
}

func TestCompileCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Compile(ctx, &model.State{}, CompileOpts{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}

func TestCompileDefaultsRevision(t *testing.T) {
	st := loadDefaults(t)
	clk := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	res, err := Compile(context.Background(), st, CompileOpts{Now: clk, Generation: 0})
	if err != nil {
		t.Fatal(err)
	}
	if res.Canonical == st {
		t.Fatal("Compile must not retain the caller pointer")
	}
	wantRev, err := config.Revision(st)
	if err != nil {
		t.Fatal(err)
	}
	if res.Revision != wantRev || res.BootstrapRevision != wantRev {
		t.Fatalf("revision=%s bootstrap=%s want %s", res.Revision, res.BootstrapRevision, wantRev)
	}
	if res.CompiledAt != clk {
		t.Fatalf("CompiledAt=%s", res.CompiledAt)
	}
	if res.Canonical.Spec.Listeners.SMTP.Address != config.DefaultSMTPAddress {
		t.Fatalf("smtp listen %q", res.Canonical.Spec.Listeners.SMTP.Address)
	}
}

func TestCompileDeterministicForSameCanonicalJSON(t *testing.T) {
	st := loadDefaults(t)
	clk := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	a, err := Compile(context.Background(), st, CompileOpts{Now: clk})
	if err != nil {
		t.Fatal(err)
	}
	st2 := loadDefaults(t)
	b, err := Compile(context.Background(), st2, CompileOpts{Now: clk})
	if err != nil {
		t.Fatal(err)
	}
	if a.Revision != b.Revision {
		t.Fatalf("revision drifted\n%s\n%s", a.Revision, b.Revision)
	}
	ja, err := config.CanonicalJSON(a.Canonical)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := config.CanonicalJSON(b.Canonical)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ja, jb) {
		t.Fatal("canonical JSON differed")
	}
}

func TestCompileDoesNotMutateInput(t *testing.T) {
	st := loadDefaults(t)
	before := st.Spec.SMTP.Hostname
	res, err := Compile(context.Background(), st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	st.Spec.SMTP.Hostname = "mutated"
	if res.Canonical.Spec.SMTP.Hostname != before && before != "" {
		if res.Canonical.Spec.SMTP.Hostname == "mutated" {
			t.Fatal("Compile mutated caller state into Canonical")
		}
	}
}

func TestCompileInvalidRejected(t *testing.T) {
	st := &model.State{
		APIVersion: model.APIVersionV1Alpha1,
		Kind:       model.KindLabMail,
		Metadata:   model.Metadata{Name: "bad"},
		Spec: model.Spec{
			SMTP: model.SMTPSpec{TLS: model.SMTPTLSSpec{Mode: model.TLSModeImplicit}},
		},
	}
	_, err := Compile(context.Background(), st, CompileOpts{})
	if err == nil {
		t.Fatal("implicit compiled")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("err=%v", err)
	}
}

func loadDefaults(t *testing.T) *model.State {
	t.Helper()
	root := repoRoot(t)
	st, err := config.LoadFile(filepath.Join(root, "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
