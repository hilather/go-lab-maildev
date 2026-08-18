// Package mcp is the MCP transport adapter for the shared capability registry.
//
// It wraps the official Go SDK (github.com/modelcontextprotocol/go-sdk v1.7.0)
// and exposes Streamable HTTP at POST /mcp for protocol 2026-07-28. Tools and
// resources are registered from internal/capabilities. Handlers call
// app.Service and contain no store or SMTP mutation logic. Errors use
// capabilities.JSONRPCFrom so data.code matches REST.
//
// Health live/ready, OpenAPI, UI assets, session/CSRF, /v1/metrics, /email
// compat, /healthz, /config, and preview are not tools. Stdio is a developer
// adapter (stdout = protocol, stderr = logs).
//
//go:generate go run ../../../scripts/generate
package mcp
