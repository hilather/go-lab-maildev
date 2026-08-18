// Package auth is the shared lab-static-bearer verifier for REST, MCP, and
// the maildev compat adapter.
//
// Bearer is primary. HTTP Basic (when mode is bearer_and_basic) maps onto
// the same token principal via tokenRef. MCP must call Authenticate with
// AllowBasic=false. Tokens are compared as SHA-256 digests; the bootstrap
// file is the only durable secret.
package auth
