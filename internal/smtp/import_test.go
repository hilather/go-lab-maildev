package smtp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var receiveOnlyRoots = []string{
	"internal/smtp",
	"internal/store",
	"internal/app",
}

func TestReceiveOnlyImportBoundary(t *testing.T) {
	root := moduleRoot(t)
	forbiddenImports := []string{
		"net/smtp",
		"net/http",
		"github.com/emersion/go-smtp",
		"github.com/hilather/go-lab-maildev/internal/control",
		"github.com/hilather/go-lab-maildev/internal/smtptest",
		"github.com/hilather/go-lab-maildev/internal/web",
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(mustRel(root, path))
		if !inReceiveOnly(rel) {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			ipath := strings.Trim(imp.Path.Value, `"`)
			for _, p := range forbiddenImports {
				if ipath == p || strings.HasPrefix(ipath, p+"/") {
					t.Errorf("%s imports forbidden %q", rel, ipath)
				}
			}
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(src)
		if strings.Contains(text, "net.Dial") || strings.Contains(text, "Dialer") {
			t.Errorf("%s contains a forbidden outbound ident", rel)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			name, ok := outboundCallName(n)
			if ok {
				t.Errorf("%s references %s", rel, name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var outboundSelectors = map[string]bool{
	"Dial": true, "DialTimeout": true, "DialContext": true,
}

func outboundCallName(n ast.Node) (string, bool) {
	switch x := n.(type) {
	case *ast.SelectorExpr:
		if x.Sel != nil && outboundSelectors[x.Sel.Name] {
			return x.Sel.Name, true
		}
	case *ast.CallExpr:
		id, ok := x.Fun.(*ast.Ident)
		if ok && outboundSelectors[id.Name] {
			return id.Name, true
		}
	}
	return "", false
}

func inReceiveOnly(rel string) bool {
	for _, p := range receiveOnlyRoots {
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

func mustRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func moduleRoot(t *testing.T) string {
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
