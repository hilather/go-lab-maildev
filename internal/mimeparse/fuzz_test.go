package mimeparse

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParse(f *testing.F) {
	dir, err := os.Getwd()
	if err == nil {
		for {
			p := filepath.Join(dir, "testdata", "mime")
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				ents, err := os.ReadDir(p)
				if err == nil {
					for _, e := range ents {
						if e.IsDir() {
							continue
						}
						b, err := os.ReadFile(filepath.Join(p, e.Name()))
						if err == nil {
							f.Add(b)
						}
					}
				}
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	f.Add([]byte(""))
	f.Add([]byte("Subject: x\r\n\r\nbody"))
	f.Fuzz(func(t *testing.T, in []byte) {
		msg := Parse(in)
		if msg == nil {
			t.Fatal("nil message")
		}
		if len(msg.Raw) != len(in) {
			t.Fatalf("raw len %d want %d", len(msg.Raw), len(in))
		}
	})
}
