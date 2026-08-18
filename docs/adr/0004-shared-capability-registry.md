# ADR 0004: Shared capability registry for REST and MCP

Status: Accepted
Date: 2026-08-17
Decisions: D4

## Context

Independent adapters tend to drift in schema, defaults, authorization, errors, and audit behavior. Today’s maildev has no MCP at all; that is a gap to close, not a constraint to keep (`mcp-integration-lab` AGENTS.md rule 8: new services expose MCP).

## Decision

**D4 — REST and MCP share one capability registry.**

- Declare every public application capability once in `internal/capabilities`.
- Bind REST (`internal/control/rest`) and MCP (`internal/control/mcp`) as adapters over `internal/app.Service`.
- The maildev `/email` compat adapter also calls `app.Service`; it does not contain store/SMTP business logic and does not call REST or MCP.
- Adapters never call each other.
- Generate `api/capabilities/v1.json`, `api/openapi/v1.json`, `api/mcp/v1.json` from the registry.
- Frozen IDs, paths, tools, and scopes live in [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md).

## Consequences

- Strong semantic parity.
- Shared authorization and mutation semantics.
- Transport-specific envelopes remain in adapters (`REST_ONLY_PROTOCOL`, `PARITY_DIFFERENT_BINDING`).
- SMTP insert stays on the data plane, not the capability registry.

## Alternatives considered

- REST-first with MCP proxying HTTP: simple but loses native MCP schemas/resources and complicates auth/error mapping.
- Independent implementations: rejected due to drift risk.
- Keep maildev’s lack of MCP: rejected by family rule 8.

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
