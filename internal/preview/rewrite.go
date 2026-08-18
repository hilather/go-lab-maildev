package preview

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/model"
)

const (
	// CSP is the frozen preview/compat-html policy (docs/06, docs/08).
	CSP = "default-src 'none'; img-src data:; style-src 'unsafe-inline'"

	// MaxPart is the largest decoded part inlined as a data: URL.
	MaxPart = 2 << 20
)

var cidRef = regexp.MustCompile(`(?i)cid:([^"'>\s]+)`)

var mediaTypeToken = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9!#$&\-^_.+]{0,126}/[a-z0-9][a-z0-9!#$&\-^_.+]{0,126}$`)

// RewriteCID inlines cid: references as data: URLs. Missing or oversized
// parts are left as cid: (broken image). HTTP attachment paths are never used.
func RewriteCID(html string, atts []model.Attachment) string {
	if html == "" {
		return html
	}
	byCID := map[string]*model.Attachment{}
	for i := range atts {
		a := &atts[i]
		for _, key := range cidKeys(a.ContentID) {
			byCID[key] = a
		}
	}
	return cidRef.ReplaceAllStringFunc(html, func(m string) string {
		sub := cidRef.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		key := normalizeCID(sub[1])
		att := byCID[key]
		if att == nil || len(att.Data) == 0 || len(att.Data) > MaxPart {
			return m
		}
		return "data:" + dataType(att.ContentType) + ";base64," + base64.StdEncoding.EncodeToString(att.Data)
	})
}

func dataType(ct string) string {
	ct = strings.TrimSpace(strings.Split(ct, ";")[0])
	if !mediaTypeToken.MatchString(ct) {
		return "application/octet-stream"
	}
	return strings.ToLower(ct)
}

func cidKeys(cid string) []string {
	n := normalizeCID(cid)
	if n == "" {
		return nil
	}
	return []string{n, strings.ToLower(n)}
}

func normalizeCID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToLower(s), "cid:")
	s = strings.Trim(s, "<>")
	return s
}
