package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/smtptest"
)

func TestTranscripts(t *testing.T) {
	dir := filepath.Dir(testdataSMTP(t, "happy-path.txt"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := startServer(t, defaultSMTPSpec(t), nil)
	addr := srv.Addr().String()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".txt" {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			smtptest.PlayTranscript(t, addr, filepath.Join(dir, name))
		})
	}
}
