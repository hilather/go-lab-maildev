# ADR 0001: Use Go

Status: Accepted
Date: 2026-08-17

## Context

The lab’s first-party appliances (LabDNS, LabLDAP control plane, TacLab) are Go. MailDev is Node. The lab wants a native sink that builds like the others, runs from a scratch/distroless image, and exposes MCP with the same SDK patterns.

## Decision

Implement LabMail in Go (`github.com/hilather/go-lab-maildev`, binary `labmaild`). Frontend remains TypeScript/React (vendored).

## Alternatives considered

- Keep Node MailDev and wrap MCP in a sidecar — rejected: two images, relay still exists upstream, REST/MCP parity would be a proxy.
- Rewrite UI in Go templates — rejected: worse UX than MailDev 3.
- Rust — rejected: team and sibling-appliance consistency.

## Consequences

Single static binary (plus embedded SPA). Agents can follow the same package-boundary rules as LabDNS/TacLab.

## Compatibility impact

Operators replace a Node image with a Go image; ports and REST paths stay.

## Migration

Image swap in mcp-integration-lab.

## Test impact

`go test`, `-race`, `govulncheck`.

## Documentation impact

This pack; README.

## Review triggers

None expected.
