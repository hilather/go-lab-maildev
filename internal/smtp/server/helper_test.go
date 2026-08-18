package server

import (
	"context"
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
