package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdataConfig(t *testing.T, elem ...string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			parts := append([]string{dir, "testdata", "config"}, elem...)
			return filepath.Join(parts...)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "labmail") {
		t.Fatalf("version output %q missing labmail", stdout.String())
	}
}

func TestUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr %q missing usage", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout.String(), "version") {
		t.Fatalf("help %q missing version", stdout.String())
	}
	if strings.Contains(stdout.String(), "SMTP listener bound") {
		t.Fatalf("help must not claim an SMTP listener")
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "not-a-command"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestServeRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "serve"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--config") {
		t.Fatalf("stderr %q missing --config", stderr.String())
	}
}

func TestValidateAndCanonicalize(t *testing.T) {
	path := testdataConfig(t, "valid", "defaults.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "validate", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok revision=sha256:") {
		t.Fatalf("validate output %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labmail", "canonicalize", "--config", path, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("canonicalize exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kind":"LabMail"`) {
		t.Fatalf("canonicalize output %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	bad := testdataConfig(t, "invalid", "unknown-field.yaml")
	code = run([]string{"labmail", "validate", "--config", bad}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("invalid validate exit %d want 1 stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labmail", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("missing --config exit %d want 2", code)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labmail", "canonicalize", "--config", path, "--format", "xml"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("bad format exit %d want 2", code)
	}
}
