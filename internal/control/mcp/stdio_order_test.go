package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/auth"
	"github.com/hilather/go-lab-maildev/internal/control/rest"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// TestStdioActorFollowsDemotionInEitherWiringOrder: REST and MCP share one
// verifier. Demoting the stdio token updates the stdio actor whether REST
// or MCP was constructed first.
func TestStdioActorFollowsDemotionInEitherWiringOrder(t *testing.T) {
	t.Run("rest_then_mcp", func(t *testing.T) {
		assertStdioDemotionOrder(t, true)
	})
	t.Run("mcp_then_rest", func(t *testing.T) {
		assertStdioDemotionOrder(t, false)
	})
}

func assertStdioDemotionOrder(t *testing.T, restFirst bool) {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, []byte(testBearerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "labmail.yaml")
	body := "apiVersion: labmail.dev/v1alpha1\nkind: LabMail\nmetadata:\n  name: t\nspec:\n  management:\n    auth:\n      mode: bearer\n      tokens:\n        - id: admin\n          secretFile: " + tok + "\n          role: administrator\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	v, err := auth.FromSpec(svc.Active().Canonical.Spec.Management.Auth)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.AuthenticateBearer(testBearerToken)
	if err != nil {
		t.Fatal(err)
	}
	fixed := app.Actor{
		ID: p.ID, Class: p.Class, Role: p.Role,
		Scopes: append([]string(nil), p.Scopes...), Transport: "mcp",
	}
	wire := func(mcpFirst bool) *Server {
		t.Helper()
		var mcpSrv *Server
		if mcpFirst {
			mcpSrv = newStdioMCP(t, svc, v, &fixed)
			newOrderREST(t, svc, v)
			return mcpSrv
		}
		newOrderREST(t, svc, v)
		return newStdioMCP(t, svc, v, &fixed)
	}
	s := wire(!restFirst)
	if err := s.authorizeTool(s.actorFrom(context.Background()), "mail_state_reset"); err != nil {
		t.Fatalf("precondition: stdio admin must reset: %v", err)
	}

	demoted := strings.Replace(body, "role: administrator", "role: viewer", 1)
	if err := os.WriteFile(cfgPath, []byte(demoted), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "operator", Transport: "test"}, app.ResetIn{Reason: "demote"}); err != nil {
		t.Fatal(err)
	}
	actor := s.actorFrom(context.Background())
	if actor.Role != model.RoleViewer {
		t.Fatalf("stdio actor role=%s want viewer", actor.Role)
	}
	if err := s.authorizeTool(actor, "mail_state_reset"); err == nil {
		t.Fatal("demoted stdio actor still authorizes mail_state_reset")
	}
	if err := s.authorizeTool(actor, "mail_version_get"); err != nil {
		t.Fatalf("demoted stdio actor lost mail.read: %v", err)
	}
}

func newStdioMCP(t *testing.T, svc *app.App, v *auth.Verifier, fixed *app.Actor) *Server {
	t.Helper()
	s, err := New(Config{
		Service: svc, Auth: v, FixedActor: fixed,
		StdioSecret: testBearerToken, RatePerSec: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func newOrderREST(t *testing.T, svc *app.App, v *auth.Verifier) {
	t.Helper()
	_, err := rest.New(rest.Config{Service: svc, Auth: v, RatePerSec: -1})
	if err != nil {
		t.Fatal(err)
	}
}
