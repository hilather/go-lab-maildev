# ADR 0003: Shared capability registry

Status: Accepted
Date: 2026-08-17

## Context

MailDev 3 MCP is a subset and stdio MCP HTTP-calls REST. Sibling appliances require REST and MCP to call one application layer so they cannot drift.

## Decision

Declare every public capability in `internal/capabilities`. REST and MCP adapters bind by name to `internal/app`. MCP must not be an HTTP client of REST. Parity tests are merge-blocking.

## Alternatives considered

- MCP wrapper around REST — rejected (TacLab/LabDNS rule; extra hop; auth duplication).
- MCP-only for agents, REST-only for UI — rejected: user asked that all REST be on MCP.

## Consequences

More tools than MailDev 3 (delete all, html, source, wait, reset, …). Prompts stay MCP-only; health stays REST-only.

## Compatibility impact

Tool names are `mail_*`, not `maildev_*`.

## Migration

None (new product).

## Test impact

`make test-parity`; registry completeness test.

## Documentation impact

docs/05, 06, 07.

## Review triggers

MCP protocol changes; new operator workflow.
