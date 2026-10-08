package rest

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/model"
)

const sseHeartbeat = 15 * time.Second

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	flusher, ok := flusherOf(w)
	if !ok {
		s.writeProblem(w, r, instance, asDomain(errStreaming))
		return
	}
	// Recheck only a stream whose actor came from this live cookie. Bearer
	// and dev-loopback (including a stale cookie that missed at connect)
	// leave cookieValue empty and are not tied to a session row.
	var cookieValue string
	if s.cfg.Auth != nil && s.cfg.Sessions != nil && strings.TrimSpace(r.Header.Get("Authorization")) == "" {
		if c, err := r.Cookie(auth.CookieName); err == nil && c.Value != "" {
			if _, hit := s.cfg.Sessions.View(c.Value); hit {
				cookieValue = c.Value
			}
		}
	}
	// Register fan-out before the client observes 200. Events are not
	// replayed; TestEventsStream inserts as soon as Do returns.
	ch, cancel := s.svc.Subscribe(r.Context(), actor, 32)
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	idle := make(chan app.InboxEvent)
	events := (<-chan app.InboxEvent)(idle)
	if ch != nil {
		events = ch
	}

	hb := s.sseHeartbeat
	if hb <= 0 {
		hb = sseHeartbeat
	}
	tick := time.NewTicker(hb)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if !s.sessionStreamLive(cookieValue) {
				return
			}
			_, _ = w.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		case ev, ok := <-events:
			if !ok {
				events = idle
				continue
			}
			if !s.sessionStreamLive(cookieValue) {
				return
			}
			payload, err := json.Marshal(sseEventJSON{
				ID: ev.ID, Subject: ev.Subject, StoreGeneration: ev.Generation,
			})
			if err != nil {
				continue
			}
			_, _ = w.Write([]byte("event: " + ev.Type + "\n"))
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(payload)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

// sessionStreamLive is true when this stream is not cookie-backed, or the
// captured cookie still views as a session with mail.read. mail.admin
// satisfies mail.read. A miss closes the loop with no further body write.
func (s *Server) sessionStreamLive(cookieValue string) bool {
	if cookieValue == "" {
		return true
	}
	if s.cfg.Sessions == nil {
		return false
	}
	sess, ok := s.cfg.Sessions.View(cookieValue)
	if !ok || !auth.HasScope(sess.Scopes, model.ScopeMailRead) {
		return false
	}
	return true
}

func flusherOf(w http.ResponseWriter) (http.Flusher, bool) {
	if f, ok := w.(http.Flusher); ok {
		return f, true
	}
	if sw, ok := w.(*statusWriter); ok {
		if f, ok := sw.ResponseWriter.(http.Flusher); ok {
			return f, true
		}
	}
	return nil, false
}

type streamErr string

func (e streamErr) Error() string { return string(e) }

const errStreaming streamErr = "streaming unsupported"
