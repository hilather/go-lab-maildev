package compiler

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

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
