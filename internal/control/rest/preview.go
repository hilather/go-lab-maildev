package rest

import (
	"net/http"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/preview"
)

const previewCSP = preview.CSP

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor, id string) {
	msg, err := s.svc.GetMessage(r.Context(), actor, id, false)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	body := preview.RewriteCID(msg.HTML, msg.Attachments)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", previewCSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
