package compat

import "github.com/hilather/go-lab-maildev/internal/auth"

// checkOrigin matches the native REST/MCP rule.
func checkOrigin(origin string, allowlist []string) error {
	return auth.CheckOrigin(origin, allowlist)
}

func (h *Handler) originAllowlist() []string {
	if h.cfg.OriginAllowlist != nil {
		return h.cfg.OriginAllowlist()
	}
	return h.cfg.AllowedOrigins
}
