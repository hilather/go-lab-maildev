// Package rest is the REST transport adapter for the shared capability registry.
//
// Routes are registered from internal/capabilities (catalog spellings only).
// Handlers call app.Service and contain no store or SMTP mutation logic.
// Errors are capabilities.ProblemFrom → application/problem+json.
// Session cookie/CSRF routes are registered from the capability catalog.
//
//go:generate go run ../../../scripts/generate
package rest
