# ADR 0006: Pin MCP protocol 2026-07-28 with a legacy-client knob

Status: Accepted
Date: 2026-08-17

## Context

LabDNS pins `2026-07-28`; MCPJungle’s client is an older generation, so the lab patches LabDNS and sets TacLab `allow_legacy_clients`. MailDev 3 MCP does not pin this lab constraint.

## Decision

Speak MCP **2026-07-28** via the official Go SDK behind an adapter. Config `mcp.allowLegacyClients` defaults **true** in lab examples and **false** in hardened examples. Record supported versions in `/v1/version`.

## Alternatives considered

- Track SDK main — nondeterministic.
- Custom MCP — rejected unless the SDK cannot serve Streamable HTTP.
- No legacy knob — would not work behind current MCPJungle without a lab patch.

## Consequences

Same operational story as TacLab. `subscriptions/listen` (if added later) may stay strict.

## Compatibility impact

MCPJungle works when the knob is on.

## Migration

When MCPJungle speaks 2026-07-28, flip lab default to false.

## Test impact

Protocol version header tests; legacy on/off.

## Documentation impact

docs/07, lab integration.

## Review triggers

Gateway upgrade; new MCP spec.
