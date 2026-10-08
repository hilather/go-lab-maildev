package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestSessionCreateAndLookup(t *testing.T) {
	s := NewStore(DefaultSessionConfig())
	p := Principal{ID: "admin", Role: model.RoleAdministrator, Scopes: allScopes()}
	cookie, csrf, sess, err := s.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cookie) < 64 || len(csrf) < 64 || sess.ID == "" {
		t.Fatalf("entropy cookie=%d csrf=%d id=%q", len(cookie), len(csrf), sess.ID)
	}
	if cookie == csrf || cookie == sess.ID {
		t.Fatal("cookie, csrf, and public id must differ")
	}
	got, gotCSRF, ok := s.Lookup(cookie)
	if !ok || got.TokenID != "admin" || gotCSRF != csrf {
		t.Fatalf("lookup %+v %q %v", got, gotCSRF, ok)
	}
}

func TestSessionExpiryAndCSRF(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	s := NewStore(SessionConfig{Idle: time.Minute, Absolute: 5 * time.Minute, Max: 8})
	s.SetClock(func() time.Time { return now })
	cookie, csrf, _, err := s.Create(Principal{ID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if !s.ValidCSRF(cookie, csrf) || s.ValidCSRF(cookie, "wrong") {
		t.Fatal("csrf compare")
	}
	now = now.Add(61 * time.Second)
	if s.ValidCSRF(cookie, csrf) {
		t.Fatal("expired session csrf accepted")
	}
	if _, _, ok := s.Lookup(cookie); ok {
		t.Fatal("idle expiry should invalidate")
	}
}

func TestSessionViewDoesNotSlide(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	s := NewStore(SessionConfig{Idle: time.Hour, Absolute: 2 * time.Hour, Max: 4})
	s.SetClock(func() time.Time { return now })
	cookie, _, sess, err := s.Create(Principal{ID: "admin", Role: model.RoleAdministrator, Scopes: allScopes()})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Minute)
	got, ok := s.View(cookie)
	if !ok {
		t.Fatal("view miss")
	}
	if !got.LastSeen.Equal(sess.LastSeen) {
		t.Fatalf("view slid LastSeen from %s to %s", sess.LastSeen, got.LastSeen)
	}
	look, _, ok := s.Lookup(cookie)
	if !ok || !look.LastSeen.Equal(now) {
		t.Fatalf("lookup LastSeen=%s want %s ok=%v", look.LastSeen, now, ok)
	}
	now = now.Add(61 * time.Minute)
	if _, ok := s.View(cookie); ok {
		t.Fatal("expired view hit")
	}
}

func TestSessionCookieFlags(t *testing.T) {
	c := NewSessionCookie("abc", false, 0)
	if c.Name != CookieName || c.Path != "/" || !c.HttpOnly {
		t.Fatalf("cookie = %+v", c)
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("samesite = %v", c.SameSite)
	}
	if c.Secure {
		t.Fatal("secure set on cleartext")
	}
	if !NewSessionCookie("abc", true, 0).Secure {
		t.Fatal("secure not set for TLS")
	}
	clear := ClearSessionCookie(true)
	if clear.MaxAge != -1 || !clear.Secure || !clear.HttpOnly {
		t.Fatalf("clear = %+v", clear)
	}
}
