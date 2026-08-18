package rest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/model"
)

const testBearerToken = "0123456789abcdef0123456789abcdef"

func newAuthServer(t *testing.T) (*Server, *app.App, *auth.Verifier) {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(dir, "pass")
	if err := os.WriteFile(pw, []byte("lab-web-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer_and_basic\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n      basic:\n        username: admin\n        passwordFile: " + pw + "\n        tokenRef: admin\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(t.Context(), app.Options{BootstrapPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{Service: svc, Auth: v, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	return s, svc, v
}

func TestUnauthenticatedMessages401(t *testing.T) {
	s, _, _ := newAuthServer(t)
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/messages", "")
	requireProblem(t, got, http.StatusUnauthorized, "unauthenticated")
	if got.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate")
	}
}

func TestNoOAuthPRM(t *testing.T) {
	s, _, _ := newAuthServer(t)
	got := doReq(t, s.Handler(), http.MethodGet, "/.well-known/oauth-protected-resource", "")
	requireProblem(t, got, http.StatusNotFound, "not_found")
}

func TestSessionCookieAndCSRF(t *testing.T) {
	s, svc, _ := newAuthServer(t)
	h := s.Handler()

	req := httptestReq(http.MethodPost, "/v1/session", "")
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	rec := doRaw(h, req)
	requireStatus(t, rec, http.StatusOK)
	m := decodeJSON(t, rec)
	csrf, _ := m["csrf"].(string)
	if len(csrf) < 64 || m["expiresAt"] == "" {
		t.Fatalf("session body=%s", rec.Body.String())
	}
	var cookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			cookie = c.Value
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Secure {
				t.Fatalf("cookie flags %+v", c)
			}
		}
	}
	if cookie == "" {
		t.Fatal("missing labmail_session")
	}

	get := httptestReq(http.MethodGet, "/v1/session", "")
	get.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	grec := doRaw(h, get)
	requireStatus(t, grec, http.StatusOK)
	gm := decodeJSON(t, grec)
	if gm["id"] != "admin" || gm["role"] != model.RoleAdministrator {
		t.Fatalf("get session=%s", grec.Body.String())
	}

	insertMail(t, svc, "sess", "x")
	// Cookie mutation without CSRF is 403.
	del := httptestReq(http.MethodDelete, "/v1/messages", "")
	del.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	drec := doRaw(h, del)
	requireProblem(t, drec, http.StatusForbidden, "forbidden")

	del = httptestReq(http.MethodDelete, "/v1/messages", "")
	del.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	del.Header.Set(auth.CSRFHeader, csrf)
	drec = doRaw(h, del)
	requireStatus(t, drec, http.StatusOK)

	logout := httptestReq(http.MethodDelete, "/v1/session", "")
	logout.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	logout.Header.Set(auth.CSRFHeader, csrf)
	lrec := doRaw(h, logout)
	if lrec.Code != http.StatusNoContent {
		t.Fatalf("logout=%d %s", lrec.Code, lrec.Body.String())
	}

	again := httptestReq(http.MethodGet, "/v1/session", "")
	again.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	arec := doRaw(h, again)
	requireProblem(t, arec, http.StatusUnauthorized, "unauthenticated")
}

func TestAuditRecordsAuthIdentity(t *testing.T) {
	s, svc, _ := newAuthServer(t)
	id := insertMail(t, svc, "audit-me", "x")
	req := httptestReq(http.MethodDelete, "/v1/messages/"+id, "")
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	rec := doRaw(s.Handler(), req)
	requireStatus(t, rec, http.StatusNoContent)

	list := httptestReq(http.MethodGet, "/v1/audit", "")
	list.Header.Set("Authorization", "Bearer "+testBearerToken)
	got := doRaw(s.Handler(), list)
	requireStatus(t, got, http.StatusOK)
	if !strings.Contains(got.Body.String(), `"actorId":"admin"`) {
		t.Fatalf("audit missing actor: %s", got.Body.String())
	}
}

func TestBasicMapsToSamePrincipal(t *testing.T) {
	s, _, _ := newAuthServer(t)
	req := httptestReq(http.MethodGet, "/v1/session", "")
	req.SetBasicAuth("admin", "lab-web-pass")
	rec := doRaw(s.Handler(), req)
	requireStatus(t, rec, http.StatusOK)
	if decodeJSON(t, rec)["id"] != "admin" {
		t.Fatalf("basic principal=%s", rec.Body.String())
	}
}

func TestHealthSkipsAuth(t *testing.T) {
	s, _, _ := newAuthServer(t)
	requireStatus(t, doReq(t, s.Handler(), http.MethodGet, "/v1/health/live", ""), http.StatusOK)
	requireStatus(t, doReq(t, s.Handler(), http.MethodGet, "/v1/health/ready", ""), http.StatusOK)
}
