package compat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

const (
	headerRequestID  = "X-Request-ID"
	headerAllow      = "Allow"
	requestURNPrefix = "urn:labmail:request:"

	// listPage is one native ListMessages page. Compat concatenates pages so
	// GET /email can return the full inbox (maildev style) up to store.maxMessages.
	listPage = 200

	textPrefixBytes = 2 << 10
)

// Principal injects the request actor. Tests may override Auth.
type Principal func(*http.Request) app.Actor

// Config constructs the maildev compat adapter.
type Config struct {
	// Service is required. Handlers call it and do not mutate snapshots.
	Service app.Service
	// Principal, when set, wins over Auth (unit goldens).
	Principal Principal
	// Auth is the shared verifier. Nil keeps goldens stub-open.
	Auth *auth.Verifier
	// AllowedOrigins is the test-only fallback when OriginAllowlist is nil.
	// Extra Origins plus sentinels "*" / "private". Empty denies non-loopback.
	AllowedOrigins []string
	// OriginAllowlist, when set, is the SoT (production: live snapshot).
	OriginAllowlist func() []string
	// Ready overrides readiness for GET /healthz. Nil is app.Status.Ready.
	Ready func() bool
}

// Handler serves /email, /healthz, and /config.
type Handler struct {
	cfg Config
	svc app.Service
}

// New builds a Handler. Service is required.
func New(cfg Config) (*Handler, error) {
	if cfg.Service == nil {
		return nil, errors.New("compat: Service is required")
	}
	return &Handler{cfg: cfg, svc: cfg.Service}, nil
}

// Mounts are the management-listener paths to register ahead of native /v1.
func (h *Handler) Mounts() map[string]http.Handler {
	if h == nil {
		return nil
	}
	// Register /email and /email/ so ServeMux does not redirect between them.
	return map[string]http.Handler{
		"/email":   h,
		"/email/":  h,
		"/healthz": h,
		"/config":  h,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)
	instance := requestURNPrefix + reqID

	if err := checkOrigin(r.Header.Get("Origin"), h.originAllowlist()); err != nil {
		h.writeProblem(w, r, instance, err)
		return
	}
	if r.Method == http.MethodOptions {
		h.writeProblem(w, r, instance, domainerr.Forbidden("CORS is disabled"))
		return
	}

	parts := splitPath(r.URL.Path)
	isHealth := len(parts) == 1 && parts[0] == "healthz"
	var actor app.Actor
	if !isHealth {
		var err error
		actor, err = h.authenticate(r)
		if err != nil {
			h.writeProblem(w, r, instance, err)
			return
		}
		if err := h.authorize(actor, writeMethod(r.Method)); err != nil {
			h.writeProblem(w, r, instance, err)
			return
		}
	}
	// Relay is a path segment after /email/{id}/, not a substring. Attachment
	// names like relay.pdf must still download.
	if isRelayPath(parts) {
		h.writeProblem(w, r, instance, domainerr.ReceiveOnly("LabMail is receive-only; relay is not implemented"))
		return
	}
	switch {
	case len(parts) == 1 && parts[0] == "healthz":
		h.requireMethod(w, r, instance, http.MethodGet, func() { h.handleHealthz(w, r, instance) })
	case len(parts) == 1 && parts[0] == "config":
		h.requireMethod(w, r, instance, http.MethodGet, func() { h.handleConfig(w, r, instance, actor) })
	case len(parts) == 1 && parts[0] == "email":
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r, instance, actor)
		default:
			h.methodNotAllowed(w, r, instance, http.MethodGet)
		}
	case len(parts) == 2 && parts[0] == "email" && parts[1] == "all":
		h.requireMethod(w, r, instance, http.MethodDelete, func() { h.handleClear(w, r, instance, actor) })
	case len(parts) == 2 && parts[0] == "email":
		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, instance, actor, parts[1])
		case http.MethodDelete:
			h.handleDelete(w, r, instance, actor, parts[1])
		default:
			h.methodNotAllowed(w, r, instance, http.MethodGet+", "+http.MethodDelete)
		}
	case len(parts) == 3 && parts[0] == "email" && parts[2] == "html":
		h.requireMethod(w, r, instance, http.MethodGet, func() { h.handleHTML(w, r, instance, actor, parts[1]) })
	case len(parts) >= 4 && parts[0] == "email" && parts[2] == "attachment":
		h.requireMethod(w, r, instance, http.MethodGet, func() {
			h.handleAttachment(w, r, instance, actor, parts[1], strings.Join(parts[3:], "/"))
		})
	default:
		h.writeProblem(w, r, instance, domainerr.NotFound("not found"))
	}
}

func (h *Handler) requireMethod(w http.ResponseWriter, r *http.Request, instance, want string, fn func()) {
	if r.Method != want {
		h.methodNotAllowed(w, r, instance, want)
		return
	}
	fn()
}

func (h *Handler) methodNotAllowed(w http.ResponseWriter, r *http.Request, instance, allow string) {
	w.Header().Set(headerAllow, allow)
	h.writeProblem(w, r, instance, domainerr.MethodNotAllowed("method not allowed"))
}

func (h *Handler) isReady(ctx context.Context) bool {
	if h.cfg.Ready != nil {
		return h.cfg.Ready()
	}
	st, err := h.svc.Status(ctx, app.Actor{ID: "ready", Class: "startup", Transport: "compat"})
	return err == nil && st != nil && st.Ready
}

func isRelayPath(parts []string) bool {
	return len(parts) >= 3 && parts[0] == "email" && strings.EqualFold(parts[2], "relay")
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func requestID(r *http.Request) string {
	if id := r.Header.Get(headerRequestID); id != "" {
		return id
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}
