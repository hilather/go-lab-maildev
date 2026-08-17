# ADR 0002: Receive-only sink

Status: Accepted
Date: 2026-08-17

## Context

MailDev can relay and auto-relay. mcp-integration-lab forbids that: the mail service exists so systems under test have somewhere to send mail, not so the lab can emit mail to the internet or other tenants.

## Decision

LabMail has no outbound SMTP client. Configuration keys and MailDev flags that would enable relay fail closed at compile/startup. Application, REST, MCP, and UI expose no send operation.

## Alternatives considered

- Implement relay behind an explicit enable flag — rejected: too easy to turn on in a shared lab.
- Stub relay endpoints that 501 — rejected: looks like a feature; 404 and omit from OpenAPI.

## Consequences

Not a MailDev substitute for developers who rely on the Relay button. Perfect for the lab.

## Compatibility impact

Relay REST/UI/CLI are non-goals. Config always reports `isOutgoingEnabled: false`.

## Migration

None; lab already rejected those flags.

## Test impact

Config tests for every relay flag/key. Architectural test: no `net/smtp.SendMail` / client send in production packages.

## Documentation impact

AGENTS.md, parity plan, security, known limitations.

## Review triggers

A product requirement to send mail (would need a new ADR and likely a different binary).
