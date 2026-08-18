// Package preview rewrites HTML for sandboxed document serving.
//
// Native GET /v1/messages/{id}/preview and compat GET /email/:id/html share
// this helper so adapters do not import each other.
package preview
