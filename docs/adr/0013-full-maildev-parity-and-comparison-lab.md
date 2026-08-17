# ADR 0013: Full MailDev functional parity + comparison lab

Status: Accepted
Date: 2026-08-17
Supersedes: [ADR 0002](0002-receive-only-sink.md) (product “no outbound client”), [ADR 0011](0011-omit-outbound-relay.md)

## Context

ADR 0002/0011 omitted relay so mcp-integration-lab could not send mail and so REST/MCP would not grow a send tool.

A later requirement: **port all MailDev functionality** and prove it with a **Docker lab running original MailDev and LabMail side by side**, compared via **REST and UI**.

That is incompatible with omitting relay, auto-relay, and the Relay UI. MailDev 2.2.1 and 3.0 both implement them.

mcp-integration-lab still must not *enable* outbound mail on the shared lab. That is a **deployment** constraint, not a missing code path.

## Decision

1. LabMail implements the MailDev inspection **and** outgoing/auto-relay surface (CLI/env/YAML, REST `POST .../relay`, UI Relay, MCP tools with the same semantics). Default config has outgoing **off** (`isOutgoingEnabled: false`), matching MailDev defaults.
2. The **comparison lab** ([docs/22-comparison-lab.md](../22-comparison-lab.md)) runs MailDev 2.2.1, MailDev 3.0 (pinned SHA), LabMail, and a captive `relay-sink`. Relay tests only target that sink. No public MTA.
3. A feature is done only when the [behavior matrix](../23-behavior-parity-matrix.md) REST case (and UI case when applicable) passes on the listed oracles **and** LabMail.
4. mcp-integration-lab continues to reject `outgoing-*` / `auto-relay*` in *its* orchestrator. LabMail **accepts** those flags so the comparison lab and anyone using LabMail as a MailDev replacement can turn them on. Example compose for the integration lab omits them.

## Alternatives considered

- Keep omitting relay and only compare ingest/list — rejected: user required all functionality and UI/REST parity.
- Implement relay but 404 it unless a build tag — rejected: MailDev does not work that way; comparison would fail.
- Relay only in tests via an extra binary — rejected: two products.

## Consequences

- `internal/relay` (or smtpd client) exists. Threat model: default off; comparison net isolated; integration lab still fail-closed at orchestrator.
- REST/MCP/UI include relay ([docs/05-control-plane-and-parity.md](../05-control-plane-and-parity.md)).
- Vendored UI **keeps** Relay (ADR 0005 delta no longer removes it). Socket.IO still replaced by `/ws` (ADR 0012).
- AGENTS.md “never send mail” becomes “never send except configured outgoing, never to the public internet from CI.”

## Compatibility impact

MailDev relay clients work when outgoing is configured. mcp-integration-lab cutover unchanged if it does not pass relay flags.

## Migration

None for existing empty tree. Docs that said “relay 404” are updated in the same change as this ADR.

## Test impact

Comparison lab REST + UI cases Y1–Y7. Config tests **accept** outgoing flags. Architectural test: outgoing host in `deploy/parity-lab` is only `relay-sink`.

## Documentation impact

docs/22, docs/23, parity plan, architecture, waves CMP, AGENTS.md, frontend, security.

## Review triggers

Desire to drop MailDev relay; change of oracle pins; opening egress from the comparison lab.
