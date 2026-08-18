package audit

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditImportDAG(t *testing.T) {
	fset := token.NewFileSet()
	forbidden := []string{
		"net/http",
		"net/smtp",
		"github.com/emersion",
		"github.com/modelcontextprotocol",
		"github.com/hilather/go-lab-maildev/internal/app",
		"github.com/hilather/go-lab-maildev/internal/control",
		"github.com/hilather/go-lab-maildev/internal/smtp",
		"github.com/hilather/go-lab-maildev/internal/store",
	}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			ipath := strings.Trim(imp.Path.Value, `"`)
			for _, p := range forbidden {
				if ipath == p || strings.HasPrefix(ipath, p+"/") {
					t.Errorf("%s imports forbidden %q", path, ipath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
