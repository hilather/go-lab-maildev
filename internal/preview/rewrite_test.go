package preview

import (
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestRewriteCIDInlinesDataURL(t *testing.T) {
	got := RewriteCID(`<html><img src="cid:pic@lab"></html>`, []model.Attachment{{
		ContentID:   "<pic@lab>",
		ContentType: "image/png",
		Data:        []byte{0x89, 0x50, 0x4e, 0x47},
	}})
	if strings.Contains(got, "cid:") {
		t.Fatalf("cid not rewritten: %s", got)
	}
	if !strings.Contains(got, "data:image/png;base64,") {
		t.Fatalf("missing data url: %s", got)
	}
	if strings.Contains(got, "/attachments/") {
		t.Fatal("must not rewrite cid to HTTP attachment paths")
	}
}

func TestRewriteCIDSanitizesContentType(t *testing.T) {
	got := RewriteCID(`<html><img src="cid:x"></html>`, []model.Attachment{{
		ContentID:   "<x>",
		ContentType: `foo" onerror="alert(1)`,
		Data:        []byte{1, 2, 3},
	}})
	if strings.Contains(got, "onerror") || strings.Contains(got, `foo"`) {
		t.Fatalf("unsafe type leaked: %s", got)
	}
	if !strings.Contains(got, "data:application/octet-stream;base64,") {
		t.Fatalf("expected octet-stream fallback: %s", got)
	}
}
