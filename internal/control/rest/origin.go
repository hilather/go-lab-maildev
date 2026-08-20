package rest

import "github.com/hilather/go-lab-maildev/internal/auth"

// checkOrigin is the shared LabDNS rule (missing allowed; non-http(s) denied).
func checkOrigin(origin string, allowlist []string) error {
	return auth.CheckOrigin(origin, allowlist)
}

func (s *Server) originAllowlist() []string {
	if s.cfg.OriginAllowlist != nil {
		return s.cfg.OriginAllowlist()
	}
	return s.cfg.AllowedOrigins
}
