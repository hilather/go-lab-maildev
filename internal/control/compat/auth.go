package compat

import (
	"net/http"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/app"
)

// stubPrincipal is open in COMPAT-001. SEC-001 adds bearer + basic and 401.
// Do not treat a missing Authorization header as unauthenticated here.
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
