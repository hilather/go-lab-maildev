// Command labmail is the LabMail process entrypoint.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/hilather/go-lab-maildev/internal/buildinfo"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	switch args[1] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "-v", "--version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Current().String())
		return 0
	case "validate":
		return validateCmd(args[2:], stdout, stderr)
	case "canonicalize":
		return canonicalizeCmd(args[2:], stdout, stderr)
	case "serve":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return serveCmd(ctx, args[2:], stdout, stderr)
	case "healthcheck":
		return healthcheckCmd(args[2:], stdout, stderr)
	case "debug-status":
		return debugStatusCmd(args[2:], stdout, stderr)
	case "mcp-stdio":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return mcpStdioCmd(ctx, args[2:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	_, _ = io.WriteString(w, usageText)
}

const usageText = `usage: labmail <command>

LabMail is a receive-only SMTP lab appliance. validate and canonicalize
load a fail-closed labmail.dev/v1alpha1 document. serve binds SMTP and
native /v1, POST /mcp, and /email compat. Auth is lab static bearer (Basic maps onto the same principal).

Commands:
  version         print build and protocol metadata
  help            print this help
  validate        fail-closed YAML check (--config)
  canonicalize    emit canonical spec (--config, --format yaml|json)
  serve           load YAML, bind SMTP, /v1, /mcp, and /email (--config, --smtp-listen,
                  --management-listen, --shutdown-timeout, --pid-file)
  healthcheck     probe GET /v1/health/ready (--url)
  mcp-stdio       Streamable MCP over stdio (--config, --token-file)
`
