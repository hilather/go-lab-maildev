// Package compat is the maildev 2.2.1 /email adapter.
//
// Handlers call app.Service and contain no store or SMTP mutation logic.
// They must not import internal/control/rest or internal/control/mcp.
// Auth uses the shared verifier. Goldens may inject Principal to stay open.
package compat
