// Command labmail is the LabMail process entrypoint.
package main

import (
	"fmt"
	"io"
	"os"

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
	case "serve", "healthcheck", "mcp-stdio":
		_, _ = fmt.Fprintf(stderr, "labmail %s is not implemented yet\n", args[1])
		return 2
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
load a fail-closed labmail.dev/v1alpha1 document. SMTP, store, REST, and
MCP are not bound yet.

Commands:
  version         print build and protocol metadata
  help            print this help
  validate        fail-closed YAML check (--config)
  canonicalize    emit canonical spec (--config, --format yaml|json)

Planned (not implemented):
  serve           load YAML and bind SMTP + management
  healthcheck     probe GET /v1/health/ready
  mcp-stdio       Streamable MCP over stdio
`
