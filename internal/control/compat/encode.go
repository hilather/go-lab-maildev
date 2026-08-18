package compat

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"type":"urn:labmail:error:internal-error","title":"Internal error","status":500,"code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *Handler) writeBytes(w http.ResponseWriter, status int, contentType string, body []byte) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *Handler) writeProblem(w http.ResponseWriter, r *http.Request, instance string, err error) {
	p := capabilities.ProblemFrom(err, instance)
	if p.Status == http.StatusUnauthorized {
		basic := h.cfg.Auth != nil && h.cfg.Auth.BasicEnabled()
		for _, v := range auth.WWWAuthenticate(basic) {
			w.Header().Add("WWW-Authenticate", v)
		}
	}
	body, merr := json.Marshal(p)
	if merr != nil {
		http.Error(w, `{"type":"urn:labmail:error:internal-error","title":"Internal error","status":500,"code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", capabilities.ProblemContentType)
	w.WriteHeader(p.Status)
	_, _ = w.Write(body)
	_ = r
}

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.Internal("internal error")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// maildev 2.2.1 uses millisecond UTC; keep that width in the compat shape.
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
