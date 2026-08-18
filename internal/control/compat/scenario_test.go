package compat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/control/rest"
	smtpserver "github.com/hilather/go-lab-maildev/internal/smtp/server"
)

// TestMaildevScenarioCompat is the mcp-integration-lab smoke twin
// (maildevScenario): SendMail, unauthenticated GET /email → 401, Basic →
// array containing subject. Goldens live in testdata/compat/. The lab swap
// PR copies this name and these three assertions.
func TestMaildevScenarioCompat(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	passPath := filepath.Join(dir, "pass")
	const token = "0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passPath, []byte("lab-web-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	yaml := strings.Join([]string{
		"apiVersion: labmail.dev/v1alpha1",
		"kind: LabMail",
		"metadata:",
		"  name: smoke",
		"spec:",
		"  management:",
		"    auth:",
		"      mode: bearer_and_basic",
		"      tokens:",
		"        - id: admin",
		"          secretFile: " + tokenPath,
		"          role: administrator",
		"      basic:",
		"        username: admin",
		"        passwordFile: " + passPath,
		"        tokenRef: admin",
	}, "\n") + "\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	snap := svc.Active()
	smtpSrv, err := smtpserver.New(smtpserver.Options{
		Address:   "127.0.0.1:0",
		Spec:      snap.Canonical.Spec.SMTP,
		Store:     svc.Inbox(),
		Snapshots: svc.Snapshots(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := smtpSrv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = smtpSrv.Shutdown(context.Background()) })

	v, err := auth.FromSpec(snap.Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := New(Config{Service: svc, Auth: v, Ready: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rest.New(rest.Config{Service: svc, Auth: v, RatePerSec: -1, Mounts: ch.Mounts(), Ready: func() bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	h := rs.Handler()

	subject := "mcplab smoke TestMaildevScenarioCompat"
	msg := []byte("From: alice@lab.test\r\nTo: bob@lab.test\r\nSubject: " + subject + "\r\n\r\nhello\r\n")
	if err := smtp.SendMail(smtpSrv.Addr().String(), nil, "alice@lab.test", []string{"bob@lab.test"}, msg); err != nil {
		t.Fatal(err)
	}

	unauth := doReq(t, h, http.MethodGet, "/email", "")
	p := requireProblem(t, unauth, http.StatusUnauthorized, "unauthenticated")
	_ = p
	www := unauth.Header().Values("WWW-Authenticate")
	joined := strings.Join(www, " ")
	if !strings.Contains(joined, `Bearer realm="labmail"`) || !strings.Contains(joined, `Basic realm="labmail"`) {
		t.Fatalf("WWW-Authenticate=%v", www)
	}

	req := httptest.NewRequest(http.MethodGet, "/email", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.SetBasicAuth("admin", "lab-web-pass")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusOK)
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("list: %v body=%s", err, rec.Body.String())
	}
	found := false
	for _, it := range items {
		if it["subject"] == subject {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("subject %q not in %s", subject, rec.Body.String())
	}

	// Same principal via bearer.
	req = httptest.NewRequest(http.MethodGet, "/email", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusOK)
}
