# ADR 0007: Compat `/email` surface

Status: Accepted
Date: 2026-08-17
Decisions: D5

## Context

mcp-integration-lab smoke (`maildevScenario`) and existing `/email` clients depend on maildev 2.2.1 REST (`GET /email`, Basic auth, `subject` in the list). Native LabMail API is the family `/v1` + `/mcp` design, not a fork of maildev v3 (`/api`). Tracking v3 would import a different auth model and keep relay.

## Decision

**D5 — Native management API is `/v1` + `POST /mcp`. Maildev `/email` is a compat adapter.**

- Compat lives in `internal/control/compat` and calls `app.Service`.
- Disposition: `REST_ONLY_PROTOCOL` plus parity-required native twins (`GET /v1/messages`, etc.).
- `POST /email/:id/relay` is always 403 `receive_only`. Never implemented.
- Compat list is a JSON **array** (maildev style), bodies omitted.
- Message ids are ULIDs, not maildev 8-char ids.
- `GET /email/:id` marks read (maildev). Native `messages.get` default `markRead=false`.
- `GET /config` is a redacted LabMail shape with `receiveOnly: true`, not a clone of 2.2.1 internals.
- No maildev WebSocket.
- Catalog id stays `maildev` during the swap release (D15); product name is LabMail.

Frozen mapping: [docs/12-maildev-compat.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/12-maildev-compat.md).

## Consequences

- Smoke can stay green without rewriting `maildevScenario` assertions.
- Agents and new clients should use `/v1` and `mail_*` tools (`mail_messages_wait`).
- Documented deltas (ULID, sha256 checksum, omitted list bodies, no `stream`) must have goldens.
- Compat is not a promise to track maildev v3.

## Alternatives considered

- Native API is maildev `/email` only: would lock the family out of `/v1` + MCP parity. Rejected.
- Track maildev v3 `/api` + upstream MCP: different auth, relay still exists, lab pin is 2.2.1. Rejected.
- Drop `/email` in the same change as the image swap: breaks smoke. Rejected.

## Review triggers

Review this decision when the lab no longer has `/email` clients, when maildev v3 becomes the lab pin (it must not), or when ULID-vs-8-char id surprises block a required client.
