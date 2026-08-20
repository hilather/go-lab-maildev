package rest

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/control/compat"
	"github.com/hilather/go-lab-maildev/internal/control/mcp"
)

const liveLANOrigin = "http://192.168.1.9:1080"

func TestOriginAllowlistLiveReadThreeSurfaces(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "labmail.yaml")
	write := func(allow string) {
		t.Helper()
		body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  listeners:\n    management:\n      compatEnabled: true\n  management:\n    originAllowlist: " + allow + "\n"
		if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("[]")
	svc, err := app.Boot(t.Context(), app.Options{BootstrapPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)

	originAllowlist := func() []string {
		snap := svc.Active()
		if snap == nil || snap.Canonical == nil {
			return nil
		}
		return snap.Canonical.Spec.Management.OriginAllowlist
	}
	mcpSrv, err := mcp.New(mcp.Config{Service: svc, RatePerSec: -1, OriginAllowlist: originAllowlist})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mcpSrv.Close)
	ch, err := compat.New(compat.Config{
		Service:         svc,
		Ready:           func() bool { return true },
		OriginAllowlist: originAllowlist,
	})
	if err != nil {
		t.Fatal(err)
	}
	mounts := ch.Mounts()
	if mounts == nil {
		mounts = map[string]http.Handler{}
	}
	mounts[mcp.DefaultPath] = mcpSrv.Handler()
	s, err := New(Config{
		Service:         svc,
		RatePerSec:      -1,
		OriginAllowlist: originAllowlist,
		Mounts:          mounts,
		Ready:           func() bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	write("[\"*\"]")
	if _, err := svc.Reset(t.Context(), app.Actor{ID: "t", Transport: "direct"}, app.ResetIn{Reason: "hatch"}); err != nil {
		t.Fatal(err)
	}
	assertLiveSurfaces(t, h, http.StatusOK)

	write("[]")
	if _, err := svc.Reset(t.Context(), app.Actor{ID: "t", Transport: "direct"}, app.ResetIn{Reason: "deny"}); err != nil {
		t.Fatal(err)
	}
	assertLiveSurfaces(t, h, http.StatusForbidden)
}

func assertLiveSurfaces(t *testing.T, h http.Handler, want int) {
	t.Helper()
	live := httptestReq(http.MethodGet, "/v1/health/live", "")
	live.Header.Set("Origin", liveLANOrigin)
	got := doRaw(h, live)
	if want == http.StatusOK {
		requireStatus(t, got, http.StatusOK)
	} else {
		m := requireProblem(t, got, http.StatusForbidden, "forbidden")
		if d, _ := m["detail"].(string); d != "origin is not allowed" {
			t.Fatalf("live detail=%q", d)
		}
	}

	hz := httptestReq(http.MethodGet, "/healthz", "")
	hz.Header.Set("Origin", liveLANOrigin)
	got = doRaw(h, hz)
	if want == http.StatusOK {
		requireStatus(t, got, http.StatusOK)
	} else {
		m := requireProblem(t, got, http.StatusForbidden, "forbidden")
		if d, _ := m["detail"].(string); d != "origin is not allowed" {
			t.Fatalf("healthz detail=%q", d)
		}
	}

	discover := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + mcp.ProtocolVersion + `","io.modelcontextprotocol/clientInfo":{"name":"labmail-test","version":"dev"},"io.modelcontextprotocol/clientCapabilities":{}}}}`
	mcpReq := httptestReq(http.MethodPost, mcp.DefaultPath, discover)
	mcpReq.Header.Set("Origin", liveLANOrigin)
	mcpReq.Header.Set("Accept", "application/json, text/event-stream")
	mcpReq.Header.Set("Mcp-Protocol-Version", mcp.ProtocolVersion)
	mcpReq.Header.Set("Mcp-Method", "server/discover")
	got = doRaw(h, mcpReq)
	if want == http.StatusOK {
		if got.Code != http.StatusOK && got.Code != http.StatusAccepted {
			t.Fatalf("mcp status=%d want 200/202 body=%s", got.Code, got.Body.String())
		}
	} else {
		m := requireProblem(t, got, http.StatusForbidden, "forbidden")
		if d, _ := m["detail"].(string); d != "origin is not allowed" {
			t.Fatalf("mcp detail=%q", d)
		}
	}
}
