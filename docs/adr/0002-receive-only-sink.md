# ADR 0002: Receive-only sink

Status: **Superseded** by [ADR 0013](0013-full-maildev-parity-and-comparison-lab.md)
Date: 2026-08-17
Superseded: 2026-08-17

## Context

MailDev can relay and auto-relay. mcp-integration-lab forbids *enabling* that: the shared lab’s mail service exists so systems under test have somewhere to send mail, not so the lab emits mail to the internet.

## Original decision

LabMail has no outbound SMTP client. Relay flags fail closed. REST/MCP/UI expose no send operation.

## Why superseded

Full MailDev functional parity (REST + UI comparison lab) requires implementing outgoing/auto-relay. The integration-lab constraint remains a **deployment** default (outgoing off; orchestrator still rejects those flags). See ADR 0013.

## Residual (still true)

- Default config: outgoing disabled.
- mcp-integration-lab must not pass `outgoing-*` / `auto-relay*`.
- Comparison lab may enable relay only to `relay-sink`.
- Captured mail stays ephemeral; YAML is not a mailbox.
