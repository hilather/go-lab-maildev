# ADR 0011: Omit outbound relay from all surfaces

Status: Accepted
Date: 2026-08-17

## Context

The user asked that all REST functionality also appear on MCP. If we exposed MailDev’s relay REST, we would have to expose MCP send. That conflicts with receive-only.

## Decision

Do not implement relay on REST, MCP, CLI, YAML, or UI. Missing routes 404. OpenAPI does not list relay. UI fork removes Relay. This keeps REST/MCP parity over the **product** surface.

## Alternatives considered

- Relay on REST only — violates the user’s MCP parity rule.
- Relay on both, disabled — still a send code path.

## Consequences

Not a full MailDev clone for people who click Relay.

## Compatibility impact

Documented non-parity.

## Migration

None.

## Test impact

UI has no relay control; OpenAPI snapshot has no relay path; flags rejected.

## Documentation impact

Parity plan, evaluation.

## Review triggers

Product change to allow sending (forbidden unless ADR 0002 is reversed first).
