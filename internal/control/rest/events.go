package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/model"
)

const sseHeartbeat = 15 * time.Second

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	flusher, ok := flusherOf(w)
	if !ok {
		s.writeProblem(w, r, instance, asDomain(errStreaming))
		return
	}
	// authenticate stored the credential on the request context. Recheck it
	// inside the loop, not here: a delete between Lookup and this function
	// must still be visible, so do not View the cookie again before subscribe.
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
			if !s.streamLive(r.Context()) {
				return
			}
			_, _ = w.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		case ev, ok := <-events:
			if !ok {
				events = idle
				continue
			}
			if !s.streamLive(r.Context()) {
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

// streamLive re-authenticates the credential that opened the stream and
// requires events.stream's scopes (mail.read; mail.admin satisfies it).
// A miss ends the loop before any further body write. Verifier and View
// are separate critical sections and are not held across the select.
// A cookie lookup uses View, so it does not slide LastSeen.
func (s *Server) streamLive(ctx context.Context) bool {
	cred, ok := streamCredFrom(ctx)
	if !ok {
		// No verifier bound this request. Unit tests and unconfigured boots
		// keep the unbound stream.
		return s.cfg.Auth == nil
	}
	switch cred.kind {
	case streamCredCookie:
		return s.cookieStreamLive(cred.cookie)
	case streamCredBearer:
		return s.bearerStreamLive(cred.secret)
	case streamCredBasic:
		return s.basicStreamLive(cred)
	case streamCredLoopback:
		return s.loopbackStreamLive(cred.remote)
	default:
		return false
	}
}

func (s *Server) cookieStreamLive(cookieValue string) bool {
	if cookieValue == "" || s.cfg.Sessions == nil {
		return false
	}
	sess, ok := s.cfg.Sessions.View(cookieValue)
	if !ok {
		return false
	}
	return streamScopesOK(sess.Scopes)
}

func (s *Server) bearerStreamLive(secret string) bool {
	if s.cfg.Auth == nil || secret == "" {
		return false
	}
	p, err := s.cfg.Auth.AuthenticateBearer(secret)
	if err != nil {
		return false
	}
	return streamScopesOK(p.Scopes)
}

func (s *Server) basicStreamLive(cred streamCred) bool {
	if s.cfg.Auth == nil || cred.secret == "" {
		return false
	}
	p, err := s.cfg.Auth.Authenticate(auth.Request{
		Authorization: "Basic " + cred.secret,
		RemoteAddr:    cred.remote,
		AllowBasic:    s.cfg.Auth.BasicEnabled(),
	})
	if err != nil {
		return false
	}
	return streamScopesOK(p.Scopes)
}

// loopbackStreamLive stays open only while the live mode is still
// dev-loopback-unauth and the remote that was allowed is still loopback.
func (s *Server) loopbackStreamLive(remote string) bool {
	if s.cfg.Auth == nil {
		return false
	}
	if s.cfg.Auth.Mode() != model.MgmtAuthDevLoopbackUnauth || !auth.IsLoopback(remote) {
		return false
	}
	p, err := s.cfg.Auth.Authenticate(auth.Request{RemoteAddr: remote})
	if err != nil {
		return false
	}
	return streamScopesOK(p.Scopes)
}

func streamScopesOK(scopes []string) bool {
	cap, ok := capabilities.Lookup(capabilities.EventsStream)
	if !ok {
		return false
	}
	return auth.AuthorizeScopes(scopes, cap.RequiredScopes) == nil
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
