package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// debugStatusCmd prints Uid/CapEff from /proc/self/status so the scratch
// image can prove identity without a shell. --check-readonly fails if the
// process can create /probe-ro (read-only root contract).
func debugStatusCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("debug-status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	checkRO := fs.Bool("check-readonly", false, "fail if / is writable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	body, err := os.ReadFile("/proc/self/status")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labmail debug-status: %v\n", err)
		return 1
	}
	var uid, capeef string
	for _, line := range strings.Split(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "Uid:"):
			uid = line
			_, _ = fmt.Fprintln(stdout, line)
		case strings.HasPrefix(line, "CapEff:"):
			capeef = line
			_, _ = fmt.Fprintln(stdout, line)
		}
	}
	if uid == "" || capeef == "" {
		_, _ = fmt.Fprintln(stderr, "labmail debug-status: missing Uid or CapEff")
		return 1
	}
	if !*checkRO {
		return 0
	}
	f, err := os.OpenFile("/probe-ro", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_ = f.Close()
		_ = os.Remove("/probe-ro")
		_, _ = fmt.Fprintln(stderr, "labmail debug-status: wrote /probe-ro (root is writable)")
		return 1
	}
	return 0
}
