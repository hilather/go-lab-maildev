package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/control/mcp"
)

func mcpStdioCmd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = stdout
	fs := flag.NewFlagSet("mcp-stdio", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	tokenFile := fs.String("token-file", "", "optional bearer token file (verified in SEC-001)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labmail mcp-stdio: --config is required")
		return 2
	}
	if *tokenFile != "" {
		if _, err := os.ReadFile(*tokenFile); err != nil {
			_, _ = fmt.Fprintf(stderr, "labmail mcp-stdio: token-file: %v\n", err)
			return 1
		}
	}
	svc, err := app.Boot(ctx, app.Options{BootstrapPath: *path})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labmail mcp-stdio: load %s: %v\n", *path, err)
		return 1
	}
	defer svc.Close()
	allowLegacy := false
	if snap := svc.Active(); snap != nil && snap.Canonical != nil {
		allowLegacy = snap.Canonical.Spec.Management.MCP.AllowLegacyClients
	}
	s, err := mcp.New(mcp.Config{
		Service:            svc,
		AllowLegacyClients: allowLegacy,
		RatePerSec:         -1,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labmail mcp-stdio: %v\n", err)
		return 1
	}
	if err := s.RunStdio(ctx); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintf(stderr, "labmail mcp-stdio: %v\n", err)
		return 1
	}
	return 0
}
