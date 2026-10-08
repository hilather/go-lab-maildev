package rest

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestRESTResetUnreadableSecret(t *testing.T) {
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n"
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
	rev := svc.Active().Revision

	missing := filepath.Join(dir, "missing.token")
	next := strings.Replace(body, "role: administrator", "role: viewer", 1)
	next = strings.Replace(next, tok, missing, 1)
	if err := os.WriteFile(cfg, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	reset := httptestReq(http.MethodPost, "/v1/state:reset", `{"reason":"rotate"}`)
	reset.Header.Set("Authorization", "Bearer "+testBearerToken)
	rec := doRaw(s.Handler(), reset)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
	got := rec.Body.String()
	if !strings.Contains(got, "spec.management.auth.tokens[0].secretFile") || !strings.Contains(got, missing) {
		t.Fatalf("problem did not name the secret file field: %s", got)
	}
	if svc.Active().Revision != rev {
		t.Fatalf("revision changed %s -> %s", rev, svc.Active().Revision)
	}
	sess := httptestReq(http.MethodGet, "/v1/session", "")
	sess.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	srec := doRaw(s.Handler(), sess)
	requireStatus(t, srec, http.StatusOK)
	if decodeJSON(t, srec)["role"] != model.RoleAdministrator {
		t.Fatalf("session after refused reset=%s", srec.Body.String())
	}
	bearer := httptestReq(http.MethodGet, "/v1/session", "")
	bearer.Header.Set("Authorization", "Bearer "+testBearerToken)
	brec := doRaw(s.Handler(), bearer)
	requireStatus(t, brec, http.StatusOK)
	if decodeJSON(t, brec)["role"] != model.RoleAdministrator {
		t.Fatalf("bearer after refused reset=%s", brec.Body.String())
	}
}

func TestRESTValidateMissingSecretFile(t *testing.T) {
	s, _ := newTestServer(t)
	missing := filepath.Join(t.TempDir(), "missing.token")
	body := fmt.Sprintf(`{"state":{"apiVersion":"labmail.dev/v1alpha1","kind":"LabMail","metadata":{"name":"t"},"spec":{"management":{"auth":{"mode":"bearer","tokens":[{"id":"admin","secretFile":%q,"role":"administrator"}]}}}}}`, missing)
	rec := doReq(t, s.Handler(), http.MethodPost, "/v1/state:validate", body)
	requireProblem(t, rec, http.StatusBadRequest, "validation_failed")
	got := rec.Body.String()
	if !strings.Contains(got, "spec.management.auth.tokens[0].secretFile") || !strings.Contains(got, missing) || !strings.Contains(got, "unresolved_reference") {
		t.Fatalf("problem did not name the missing secretFile: %s", got)
	}
}
