# ADR 0011: Omit outbound relay from all surfaces

Status: **Superseded** by [ADR 0013](0013-full-maildev-parity-and-comparison-lab.md)
Date: 2026-08-17
Superseded: 2026-08-17

## Original decision

Do not implement relay on REST, MCP, CLI, YAML, or UI (404, no OpenAPI, UI fork removes Relay) so REST/MCP parity would not require a send tool.

## Why superseded

All MailDev functionality must be ported and proven in the side-by-side lab. Relay is in-scope on REST, MCP, and UI when outgoing is configured. Default remains off.

## Residual

Do not document relay as always-on. mcp-integration-lab examples omit it.
