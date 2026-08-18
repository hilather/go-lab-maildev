package compat

import "github.com/hilather/go-lab-maildev/internal/auth"

// checkOrigin matches the native REST/MCP rule.
func checkOrigin(origin string, allowlist []string) error {
	return auth.CheckOrigin(origin, allowlist)
}
