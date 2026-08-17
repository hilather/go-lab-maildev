# ADR 0006: Pin supported MCP protocol versions

Status: Accepted
Date: 2026-08-17
Decisions: D14, D17

## Context

MCP evolves and recent revisions changed transport and statelessness behavior. Claiming support for an unpinned latest version would make compatibility and testing ambiguous.

LabDNS hard-pins `2026-07-28` with no compatibility knob; `mcp-integration-lab` carries `patches/go-lab-dns-wire-mcp.patch` because MCPJungle (`mark3labs/mcp-go v0.48`) cannot speak that pin. TacLab ships `api.mcp.allow_legacy_clients` (default false; lab turns it on). LabMail is a new appliance and should not require a lab patch.

## Decision

**D14 — Go 1.26, official MCP SDK `v1.7.0`, protocol `2026-07-28`, Apache-2.0.**

- Record the pin in `internal/buildinfo` and `/v1/version`.
- Transport: Streamable HTTP `POST /mcp`, `Stateless: true`.
- Add a protocol version only after conformance and parity tests pass.

**D17 — `spec.management.mcp.allowLegacyClients` default false; integration-lab overlay sets true.**

- TacLab-shaped knob so MCPJungle can register without a LabMail patch.
- `subscriptions/listen` stays pinned to 2026-07-28 even when the pin is relaxed.
- Missing Origin is allowed (same as REST).

## Consequences

- Behavior is reproducible.
- Protocol upgrades are explicit reviewed work.
- The lab bootstrap YAML must set `allowLegacyClients: true`.
- Release notes must list MCP version changes.

## Alternatives considered

- Track SDK main automatically: rejected due to nondeterminism.
- Custom MCP implementation: rejected unless the official SDK cannot satisfy required behavior and an ADR replaces this one.
- Hard pin with no knob (LabDNS): would require a lab patch. Rejected for LabMail.
- Default `allowLegacyClients: true`: too loose for standalone / hardened deploys. Lab overlay turns it on.

## Review triggers

Review this decision when MCPJungle speaks 2026-07-28 natively, when the official SDK cannot satisfy required behavior, or when a new protocol version is proposed.
