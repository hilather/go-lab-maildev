package rest

import (
	"net/http"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/web"
)

// TestSPAAssetOriginMatrix logs hashed-JS Origin outcomes. It does not freeze
// 403-on-LAN as intended product behavior; see spa_asset_origin_hatch_test.go.
func TestSPAAssetOriginMatrix(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }
	h := s.Handler()
	js := spaHashedJS(t, h)

	cases := []struct {
		name, path, origin string
		extra              []string
	}{
		{name: "html_missing_origin", path: "/", origin: ""},
		{name: "html_lan_http_origin_basic", path: "/", origin: "http://192.168.1.9:1080"},
		{name: "html_loopback", path: "/", origin: "http://127.0.0.1:1080"},
		{name: "js_missing_origin", path: js, origin: ""},
		{name: "js_lan_default", path: js, origin: "http://192.168.1.9:1080"},
		{name: "js_loopback", path: js, origin: "http://127.0.0.1:1080"},
		{name: "js_lan_star", path: js, origin: "http://192.168.1.9:1080", extra: []string{"*"}},
		{name: "js_lan_private", path: js, origin: "http://192.168.1.9:1080", extra: []string{"private"}},
	}
	for _, tc := range cases {
		s.cfg.AllowedOrigins = tc.extra
		req := httptestReq(http.MethodGet, tc.path, "")
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := doRaw(h, req)
		t.Logf("%s path=%s origin=%q extra=%v status=%d", tc.name, tc.path, tc.origin, tc.extra, rec.Code)
	}
}
