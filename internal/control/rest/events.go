package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
)

const sseHeartbeat = 15 * time.Second

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	flusher, ok := flusherOf(w)
	if !ok {
		s.writeProblem(w, r, instance, asDomain(errStreaming))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	idle := make(chan app.InboxEvent)
	events := (<-chan app.InboxEvent)(idle)
	ch, cancel := s.svc.Subscribe(r.Context(), actor, 32)
	defer cancel()
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
			_, _ = w.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		case ev, ok := <-events:
			if !ok {
				events = idle
				continue
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
