package rest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/web"
)

func newSPAServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }
	return s, s.Handler()
}

func spaHashedJS(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := doReq(t, h, http.MethodGet, "/", "")
	requireStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	const marker = `src="`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no script src in %s", body)
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatalf("unterminated script src in %s", body)
	}
	src := rest[:j]
	if !strings.HasPrefix(src, "/assets/") || !strings.HasSuffix(src, ".js") {
		t.Fatalf("hashed js src=%q", src)
	}
	return src
}

func TestDefaultDenyLANJSOrigin(t *testing.T) {
	_, h := newSPAServer(t)
	js := spaHashedJS(t, h)

	html := doReq(t, h, http.MethodGet, "/", "")
	requireStatus(t, html, http.StatusOK)

	req := httptestReq(http.MethodGet, js, "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	m := requireProblem(t, doRaw(h, req), http.StatusForbidden, "forbidden")
	if d, _ := m["detail"].(string); d != "origin is not allowed" {
		t.Fatalf("detail=%q", d)
	}
}

func TestStarAllowsLANJSOrigin(t *testing.T) {
	s, h := newSPAServer(t)
	s.cfg.AllowedOrigins = []string{"*"}
	js := spaHashedJS(t, h)

	html := doReq(t, h, http.MethodGet, "/", "")
	requireStatus(t, html, http.StatusOK)

	lanHTML := httptestReq(http.MethodGet, "/", "")
	lanHTML.Header.Set("Origin", "http://192.168.1.9:1080")
	requireStatus(t, doRaw(h, lanHTML), http.StatusOK)

	req := httptestReq(http.MethodGet, js, "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	rec := doRaw(h, req)
	requireStatus(t, rec, http.StatusOK)
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Fatalf("content-type=%q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty js body")
	}
	requireNoACAO(t, rec)
}

func TestPrivateAllowsLANJSOrigin(t *testing.T) {
	s, h := newSPAServer(t)
	s.cfg.AllowedOrigins = []string{"private"}
	js := spaHashedJS(t, h)

	lanHTML := httptestReq(http.MethodGet, "/", "")
	lanHTML.Header.Set("Origin", "http://192.168.1.9:1080")
	requireStatus(t, doRaw(h, lanHTML), http.StatusOK)

	req := httptestReq(http.MethodGet, js, "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	requireStatus(t, doRaw(h, req), http.StatusOK)

	evil := httptestReq(http.MethodGet, js, "")
	evil.Header.Set("Origin", "https://evil.example")
	requireProblem(t, doRaw(h, evil), http.StatusForbidden, "forbidden")
}

func TestExactAllowsLANJSOrigin(t *testing.T) {
	s, h := newSPAServer(t)
	s.cfg.AllowedOrigins = []string{"http://192.168.1.9:1080"}
	js := spaHashedJS(t, h)

	req := httptestReq(http.MethodGet, js, "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	requireStatus(t, doRaw(h, req), http.StatusOK)
}

func TestSPAOptionsCORSDisabledWithStar(t *testing.T) {
	s, h := newSPAServer(t)
	s.cfg.AllowedOrigins = []string{"*"}
	js := spaHashedJS(t, h)

	req := httptestReq(http.MethodOptions, js, "")
	req.Header.Set("Origin", "http://192.168.1.9:1080")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	rec := doRaw(h, req)
	m := requireProblem(t, rec, http.StatusForbidden, "forbidden")
	if d, _ := m["detail"].(string); d != "CORS is disabled" {
		t.Fatalf("detail=%q", d)
	}
	requireNoACAO(t, rec)
}
