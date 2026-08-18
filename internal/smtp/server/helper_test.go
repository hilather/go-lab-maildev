package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/smtptest"
	"github.com/hilather/go-lab-maildev/internal/store"
)

func defaultSMTPSpec(t *testing.T) model.SMTPSpec {
	t.Helper()
	st, err := config.Load([]byte("apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	return st.Spec.SMTP
}

func startServer(t *testing.T, spec model.SMTPSpec, sink store.Sink) *Server {
	t.Helper()
	if spec.Hostname == "" && spec.MaxMessageBytes == 0 {
		spec = defaultSMTPSpec(t)
	} else {
		spec = withSpecDefaults(spec)
	}
	if sink == nil {
		sink = store.NewNull()
	}
	srv, err := New(Options{Address: "127.0.0.1:0", Spec: spec, Store: sink})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return srv
}

func dial(t *testing.T, srv *Server) *smtptest.Client {
	t.Helper()
	c, err := smtptest.Dial(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func testdataSMTP(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata", "smtp", name)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func mustCmd(t *testing.T, c *smtptest.Client, want int, line string) []string {
	t.Helper()
	code, lines, err := c.Cmd(line)
	if err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	if code != want {
		t.Fatalf("%s: code %d want %d (%v)", line, code, want, lines)
	}
	return lines
}

func authSpec(t *testing.T, user, pass string) model.SMTPSpec {
	t.Helper()
	spec := defaultSMTPSpec(t)
	spec.Auth.Mode = model.SMTPAuthPlainLogin
	spec.Auth.Username = user
	spec.Auth.PasswordFile = writeSecretFile(t, pass)
	return spec
}

func writeSecretFile(t *testing.T, pass string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "smtp.pass")
	if err := os.WriteFile(p, []byte(pass+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func starttlsSpec(t *testing.T, required bool) model.SMTPSpec {
	t.Helper()
	spec := defaultSMTPSpec(t)
	cert, key := writeTestCert(t)
	spec.TLS.Mode = model.TLSModeStartTLS
	spec.TLS.Required = required
	spec.TLS.CertFile = cert
	spec.TLS.KeyFile = key
	return spec
}

func writeTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "labmail.lab"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost", "labmail.lab"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile = filepath.Join(dir, "smtp.crt")
	keyFile = filepath.Join(dir, "smtp.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}
