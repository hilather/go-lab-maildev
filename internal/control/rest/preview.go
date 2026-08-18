package rest

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/model"
)

const (
	previewCSP = "default-src 'none'; img-src data:; style-src 'unsafe-inline'"
	previewMax = 2 << 20
)

var cidRef = regexp.MustCompile(`(?i)cid:([^"'>\s]+)`)

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor, id string) {
	msg, err := s.svc.GetMessage(r.Context(), actor, id, false)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	body := rewriteCID(msg.HTML, msg.Attachments)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", previewCSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func rewriteCID(html string, atts []model.Attachment) string {
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
		if att == nil || len(att.Data) == 0 || len(att.Data) > previewMax {
			return m
		}
		return "data:" + previewDataType(att.ContentType) + ";base64," + base64.StdEncoding.EncodeToString(att.Data)
	})
}

var mediaTypeToken = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9!#$&\-^_.+]{0,126}/[a-z0-9][a-z0-9!#$&\-^_.+]{0,126}$`)

func previewDataType(ct string) string {
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
