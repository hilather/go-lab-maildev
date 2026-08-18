package compat

import (
	"net/http"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// stubPrincipal is open when no Auth/Principal is configured (unit goldens).
func stubPrincipal(r *http.Request) app.Actor {
	actor := app.Actor{ID: "anonymous", Class: "administrator", Transport: "compat"}
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return actor
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		actor.ID = "bearer"
		return actor
	}
	if strings.HasPrefix(strings.ToLower(h), "basic ") {
		actor.ID = "basic"
		return actor
	}
	return actor
}

func actorOf(p auth.Principal) app.Actor {
	return app.Actor{
		ID:        p.ID,
		Class:     p.Class,
		Role:      p.Role,
		Scopes:    append([]string(nil), p.Scopes...),
		Transport: "compat",
	}
}

func (h *Handler) authenticate(r *http.Request) (app.Actor, error) {
	if h.cfg.Principal != nil {
		return h.cfg.Principal(r), nil
	}
	if h.cfg.Auth == nil {
		return stubPrincipal(r), nil
	}
	p, err := h.cfg.Auth.Authenticate(auth.Request{
		Authorization: r.Header.Get("Authorization"),
		RemoteAddr:    r.RemoteAddr,
		AllowBasic:    h.cfg.Auth.BasicEnabled(),
	})
	if err != nil {
		return app.Actor{}, err
	}
	return actorOf(p), nil
}

func (h *Handler) authorize(actor app.Actor, write bool) error {
	if h.cfg.Auth == nil && h.cfg.Principal == nil {
		return nil
	}
	if h.cfg.Auth == nil {
		return nil
	}
	want := model.ScopeMailRead
	if write {
		want = model.ScopeMailWrite
	}
	return auth.AuthorizeScopes(actor.Scopes, []string{want})
}

func writeMethod(method string) bool {
	return method == http.MethodDelete || method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
}
