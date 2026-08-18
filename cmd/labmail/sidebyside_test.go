package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/compatcheck"
)

const sideBySidePass = "lab-web-pass"

func TestSideBySideMaildev221(t *testing.T) {
	labSMTP, labHTTP, stopLab := startLabMailOracle(t)
	defer stopLab()

	subject := "sbs-side-by-side-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	raw := []byte("From: Alice <alice@lab.test>\r\nTo: Bob <bob@lab.test>\r\nSubject: " + subject + "\r\n\r\nhello side-by-side body\r\n")

	lab := compatcheck.Oracle{
		Name: "labmail",
		SMTP: labSMTP,
		HTTP: labHTTP,
		User: "admin",
		Pass: sideBySidePass,
	}
	labRep, err := compatcheck.Probe(lab, raw, "alice@lab.test", []string{"bob@lab.test"}, subject)
	if err != nil {
		t.Fatal(err)
	}
	if errs := compatcheck.SwapGateOK(labRep, true); len(errs) > 0 {
		t.Fatalf("labmail swap-gate: %s", strings.Join(errs, "; "))
	}
	if errs := compatcheck.DocumentedLabMailDeltas(labRep); len(errs) > 0 {
		t.Fatalf("labmail documented deltas: %s", strings.Join(errs, "; "))
	}
	t.Log(compatcheck.FormatTranscript(labRep))

	mdSMTP, mdHTTP, stopMD, envErr := startMaildevOracle(t)
	if envErr != "" {
		t.Log(envErr)
		return
	}
	defer stopMD()

	md := compatcheck.Oracle{
		Name: "maildev-2.2.1",
		SMTP: mdSMTP,
		HTTP: mdHTTP,
		User: "admin",
		Pass: sideBySidePass,
	}
	mdRep, err := compatcheck.Probe(md, raw, "alice@lab.test", []string{"bob@lab.test"}, subject)
	if err != nil {
		t.Fatal(err)
	}
	if errs := compatcheck.SwapGateOK(mdRep, false); len(errs) > 0 {
		t.Fatalf("maildev swap-gate: %s", strings.Join(errs, "; "))
	}
	if errs := compatcheck.SharedShapeDiff(mdRep, labRep); len(errs) > 0 {
		t.Fatalf("undesigned shared-shape diffs: %s", strings.Join(errs, "; "))
	}
	t.Log(compatcheck.FormatTranscript(mdRep))
	t.Logf("plain: maildev id=%v labmail id=%v subject=%q relay_md=%d relay_lm=%d",
		mdRep.ListItem["id"], labRep.ListItem["id"], subject, mdRep.RelayStatus, labRep.RelayStatus)

	attSubject := subject + "-attach"
	attRaw := []byte("From: Alice <alice@lab.test>\r\nTo: Bob <bob@lab.test>\r\nSubject: " + attSubject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=sbsbound\r\n\r\n" +
		"--sbsbound\r\nContent-Type: text/plain\r\n\r\nbody with attach\r\n" +
		"--sbsbound\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"note.pdf\"\r\n\r\n%PDF-1.4\r\n" +
		"--sbsbound--\r\n")
	labAtt, err := compatcheck.Probe(lab, attRaw, "alice@lab.test", []string{"bob@lab.test"}, attSubject)
	if err != nil {
		t.Fatal(err)
	}
	mdAtt, err := compatcheck.Probe(md, attRaw, "alice@lab.test", []string{"bob@lab.test"}, attSubject)
	if err != nil {
		t.Fatal(err)
	}
	if errs := compatcheck.SwapGateOK(labAtt, true); len(errs) > 0 {
		t.Fatalf("labmail attach swap-gate: %s", strings.Join(errs, "; "))
	}
	if errs := compatcheck.SwapGateOK(mdAtt, false); len(errs) > 0 {
		t.Fatalf("maildev attach swap-gate: %s", strings.Join(errs, "; "))
	}
	if errs := compatcheck.DocumentedLabMailDeltas(labAtt); len(errs) > 0 {
		t.Fatalf("labmail attach deltas: %s", strings.Join(errs, "; "))
	}
	if errs := compatcheck.SharedShapeDiff(mdAtt, labAtt); len(errs) > 0 {
		t.Fatalf("undesigned attachment diffs: %s", strings.Join(errs, "; "))
	}
	t.Logf("attach: maildev names=%v labmail names=%v", mdAtt.GetItem["attachments"], labAtt.GetItem["attachments"])
}

func startLabMailOracle(t *testing.T) (smtpAddr, httpAddr string, stop func()) {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	pw := filepath.Join(dir, "pass")
	if err := os.WriteFile(tok, []byte(serveTestToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pw, []byte(sideBySidePass+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	body := strings.Join([]string{
		"apiVersion: labmail.dev/v1alpha1",
		"kind: LabMail",
		"metadata:",
		"  name: sbs",
		"spec:",
		"  management:",
		"    auth:",
		"      mode: bearer_and_basic",
		"      tokens:",
		"        - id: admin",
		"          secretFile: " + tok,
		"          role: administrator",
		"      basic:",
		"        username: admin",
		"        passwordFile: " + pw,
		"        tokenRef: admin",
		"  observability:",
		"    metrics:",
		"      listen: \"\"",
	}, "\n") + "\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	rt, err := serveFromConfig(ctx, serveFlags{
		Config:           cfg,
		SMTPListen:       "127.0.0.1:0",
		ManagementListen: "127.0.0.1:0",
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stop = func() {
		shctx, done := context.WithTimeout(context.Background(), 3*time.Second)
		_ = rt.shutdown(shctx)
		done()
		cancel()
	}
	smtpAddr = rt.smtp.Addr().String()
	httpAddr = rt.http.Addr()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + httpAddr + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return smtpAddr, httpAddr, stop
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	stop()
	t.Fatal("labmail /healthz never became 200")
	return "", "", stop
}

func startMaildevOracle(t *testing.T) (smtpAddr, httpAddr string, stop func(), envErr string) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		return "", "", func() {}, "docker not on PATH: " + err.Error()
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		return "", "", func() {}, "docker daemon not usable: " + err.Error()
	}
	name := fmt.Sprintf("labmail-sbs-%d", time.Now().UnixNano())
	cmd := exec.Command("docker", "run", "-d", "--name", name,
		"-p", "127.0.0.1::1025", "-p", "127.0.0.1::1080",
		"maildev/maildev:2.2.1",
		"--web-user", "admin", "--web-pass", sideBySidePass,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", func() {}, fmt.Sprintf("docker run maildev/maildev:2.2.1: %v: %s", err, out)
	}
	stop = func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	}
	smtpAddr, err = dockerHostPort(name, "1025/tcp")
	if err != nil {
		stop()
		return "", "", func() {}, "docker port smtp: " + err.Error()
	}
	httpAddr, err = dockerHostPort(name, "1080/tcp")
	if err != nil {
		stop()
		return "", "", func() {}, "docker port http: " + err.Error()
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + httpAddr + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return smtpAddr, httpAddr, stop, ""
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	stop()
	return "", "", func() {}, "maildev /healthz never became 200 on " + httpAddr
}

func dockerHostPort(name, spec string) (string, error) {
	out, err := exec.Command("docker", "port", name, spec).Output()
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	line = strings.ReplaceAll(line, "0.0.0.0", "127.0.0.1")
	line = strings.ReplaceAll(line, "[::]", "127.0.0.1")
	if line == "" {
		return "", fmt.Errorf("empty docker port for %s %s", name, spec)
	}
	return line, nil
}
