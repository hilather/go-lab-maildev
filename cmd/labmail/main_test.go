package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/smtp/server"
)

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

func testdataConfig(t *testing.T, elem ...string) string {
	t.Helper()
	parts := append([]string{repoRoot(t), "testdata", "config"}, elem...)
	return filepath.Join(parts...)
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
	if !strings.Contains(stdout.String(), "mcp-stdio") {
		t.Fatalf("help %q missing mcp-stdio", stdout.String())
	}
	if strings.Contains(stdout.String(), "Planned (not implemented)") {
		t.Fatal("help still lists mcp-stdio as planned")
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "not-a-command"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestMCPStdioRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "mcp-stdio"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--config") {
		t.Fatalf("stderr %q missing --config", stderr.String())
	}
}

func TestMCPStdioRequiresTokenFile(t *testing.T) {
	path := testdataConfig(t, "valid", "defaults.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "mcp-stdio", "--config", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d want 1 stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--token-file") {
		t.Fatalf("stderr %q missing --token-file", stderr.String())
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

func TestParseServeFlags(t *testing.T) {
	var stderr bytes.Buffer
	got, err := parseServeFlags([]string{
		"--config", "labmail.yaml",
		"--smtp-listen", "127.0.0.1:1025",
		"--management-listen", "off",
		"--shutdown-timeout", "8s",
		"--pid-file", "/tmp/labmail.pid",
	}, &stderr)
	if err != nil {
		t.Fatalf("parse: %v stderr=%q", err, stderr.String())
	}
	if got.Config != "labmail.yaml" || got.SMTPListen != "127.0.0.1:1025" {
		t.Fatalf("listen flags: %+v", got)
	}
	if got.ManagementListen != "off" {
		t.Fatalf("management-listen=%q", got.ManagementListen)
	}
	if got.ShutdownTimeout != 8*time.Second {
		t.Fatalf("shutdown-timeout=%s want 8s", got.ShutdownTimeout)
	}
	if got.PIDFile != "/tmp/labmail.pid" {
		t.Fatalf("pid-file=%q", got.PIDFile)
	}

	stderr.Reset()
	def, err := parseServeFlags([]string{"--config", "labmail.yaml"}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if def.ShutdownTimeout != server.DefaultShutdownWait {
		t.Fatalf("default shutdown=%s", def.ShutdownTimeout)
	}
}

func TestDebugStatus(t *testing.T) {
	if _, err := os.Stat("/proc/self/status"); err != nil {
		t.Skip("/proc/self/status not available")
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"labmail", "debug-status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Uid:") {
		t.Fatalf("stdout=%q missing Uid", stdout.String())
	}
	if !strings.Contains(stdout.String(), "CapEff:") {
		t.Fatalf("stdout=%q missing CapEff", stdout.String())
	}
}

func TestDockerfileHardening(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"FROM scratch",
		"USER 65532:65532",
		"Apache-2.0",
		"ghcr.io/hilather/labmail",
		`ENTRYPOINT ["/labmail"]`,
		`CMD ["serve", "--config=/etc/labmail/config.yaml"]`,
		`CMD ["/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]`,
		"EXPOSE 1025/tcp 1080/tcp",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Dockerfile missing %q", want)
		}
	}
	if strings.Contains(text, "node -e") || strings.Contains(text, `CMD ["node"`) {
		t.Error("Dockerfile healthcheck must not exec node")
	}
}

func TestComposeSmokeContract(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "examples", "compose.smoke.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		`test: ["CMD", "/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]`,
		`user: "65532:65532"`,
		"read_only: true",
		"cap_drop:",
		"- ALL",
		"tmpfs:",
		"- /tmp",
		"no-new-privileges:true",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("compose.smoke.yaml missing %q", want)
		}
	}
	if strings.Contains(text, "node -e") || strings.Contains(text, `"node"`) {
		t.Error("compose smoke healthcheck must not exec node")
	}
}

func TestExampleAndContainerYAML(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"examples/labmail.yaml",
		"examples/labmail.origin-dev.yaml",
		"testdata/container/config.yaml",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		st, err := config.LoadFile(path)
		if err != nil {
			t.Fatalf("load %s: %v", rel, err)
		}
		if rel == "testdata/container/config.yaml" {
			if len(st.Spec.Management.OriginAllowlist) != 0 {
				t.Fatalf("container smoke originAllowlist=%q", st.Spec.Management.OriginAllowlist)
			}
			if st.Spec.UI.Enabled {
				t.Fatal("container smoke ui.enabled must stay false")
			}
		}
	}
}
