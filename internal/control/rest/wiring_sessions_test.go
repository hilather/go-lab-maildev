package rest

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/control/mcp"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestServeWiringDemotionClearsSessions(t *testing.T) {
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
	// Production wiring (cmd/labmail/serve.go) builds the MCP server first on the
	// same verifier, so its OnApply/OnReset reloadAuth runs before REST's.
	m, err := mcp.New(mcp.Config{Service: svc, Auth: v, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	s, err := New(Config{Service: svc, Auth: v, Sessions: auth.NewStore(auth.DefaultSessionConfig()), RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}

	req := httptestReq(http.MethodPost, "/v1/session", "")
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	rec := doRaw(s.Handler(), req)
	requireStatus(t, rec, http.StatusOK)
	var cookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("missing session cookie")
	}

	// Demote the live spec, then Apply so OnApply (not reloadAuth directly) rebuilds.
	snap := svc.Active()
	if len(snap.Canonical.Spec.Management.Auth.Tokens) != 1 {
		t.Fatal("expected one token")
	}
	snap.Canonical.Spec.Management.Auth.Tokens[0].Role = model.RoleViewer
	snap.Canonical.Spec.Management.Auth.Tokens[0].Scopes = nil

	applyBody, err := json.Marshal(map[string]any{
		"expectedRevision": string(snap.Revision),
		"operations": []map[string]any{{
			"op":             "replaceHideExtensions",
			"hideExtensions": []string{"SIZE"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	apply := httptestReq(http.MethodPost, "/v1/changes:apply", string(applyBody))
	apply.Header.Set("Authorization", "Bearer "+testBearerToken)
	arec := doRaw(s.Handler(), apply)
	requireStatus(t, arec, http.StatusOK)

	stale := httptestReq(http.MethodGet, "/v1/session", "")
	stale.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	srec := doRaw(s.Handler(), stale)
	requireProblem(t, srec, http.StatusUnauthorized, "unauthenticated")

	bearer := httptestReq(http.MethodGet, "/v1/session", "")
	bearer.Header.Set("Authorization", "Bearer "+testBearerToken)
	brec := doRaw(s.Handler(), bearer)
	requireStatus(t, brec, http.StatusOK)
	if decodeJSON(t, brec)["role"] != model.RoleViewer {
		t.Fatalf("bearer after demotion=%s", brec.Body.String())
	}
}

// TestServeWiringBasicPasswordReplace changes only the Basic password (same
// role) and reloads through Apply. Replace must install the new digest: the
// old password fails, and the new one authenticates as that token's role.
func TestServeWiringBasicPasswordReplace(t *testing.T) {
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
	m, err := mcp.New(mcp.Config{Service: svc, Auth: v, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	s, err := New(Config{Service: svc, Auth: v, Sessions: auth.NewStore(auth.DefaultSessionConfig()), RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}

	oldAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:lab-web-pass"))
	before := httptestReq(http.MethodGet, "/v1/session", "")
	before.Header.Set("Authorization", oldAuth)
	brec := doRaw(s.Handler(), before)
	requireStatus(t, brec, http.StatusOK)
	if decodeJSON(t, brec)["role"] != model.RoleAdministrator {
		t.Fatalf("basic before rotate=%s", brec.Body.String())
	}

	login := httptestReq(http.MethodPost, "/v1/session", "")
	login.Header.Set("Authorization", "Bearer "+testBearerToken)
	lrec := doRaw(s.Handler(), login)
	requireStatus(t, lrec, http.StatusOK)
	var cookie string
	for _, c := range lrec.Result().Cookies() {
		if c.Name == auth.CookieName {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("missing session cookie")
	}

	if err := os.WriteFile(pw, []byte("new-web-pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snap := svc.Active()
	applyBody, err := json.Marshal(map[string]any{
		"expectedRevision": string(snap.Revision),
		"operations": []map[string]any{{
			"op":             "replaceHideExtensions",
			"hideExtensions": []string{"SIZE"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	apply := httptestReq(http.MethodPost, "/v1/changes:apply", string(applyBody))
	apply.Header.Set("Authorization", "Bearer "+testBearerToken)
	arec := doRaw(s.Handler(), apply)
	requireStatus(t, arec, http.StatusOK)

	stale := httptestReq(http.MethodGet, "/v1/session", "")
	stale.Header.Set("Authorization", oldAuth)
	srec := doRaw(s.Handler(), stale)
	requireProblem(t, srec, http.StatusUnauthorized, "unauthenticated")

	neu := httptestReq(http.MethodGet, "/v1/session", "")
	neu.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:new-web-pass")))
	nrec := doRaw(s.Handler(), neu)
	requireStatus(t, nrec, http.StatusOK)
	if decodeJSON(t, nrec)["role"] != model.RoleAdministrator {
		t.Fatalf("new basic role=%s", nrec.Body.String())
	}

	cookieReq := httptestReq(http.MethodGet, "/v1/session", "")
	cookieReq.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	crec := doRaw(s.Handler(), cookieReq)
	requireProblem(t, crec, http.StatusUnauthorized, "unauthenticated")
}
