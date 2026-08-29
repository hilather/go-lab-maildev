# ADR 0009: Single-message mark-read mutation

Status: Accepted
Date: 2026-08-29
Decisions: D19

## Context

Native `GET /v1/messages/{id}` defaults `markRead=false`. `markRead=true` on that GET is a write of the read bit, but `messages.get` is catalogued as non-mutating `mail.read`. Cookie CSRF runs only for `auth.UnsafeMethod` (POST/PUT/PATCH/DELETE). The inbox SPA must not mark read as a GET side effect; select must use an explicit CSRF mutation. `POST /v1/messages:read-all` is CSRF-protected but marks every message. Domain `app.MarkRead` already exists with no REST/MCP binding. Inventing a path or capability ID requires this ADR (docs/05; AGENTS.md).

## Decision

**D19 — Single-message mark-read is `messages.read`.**

- Capability `messages.read`: `POST /v1/messages/{id}:read`, MCP `mail_message_read`, scope `mail.write`, mutating, idempotent, service method `MarkRead`.
- Does not bump `storeGeneration` (existing store behavior). No audit event (same as `messages.read_all`).
- REST empty body is allowed. Success is `204`. MCP structured content is `{ok: true}`.
- `GET /v1/messages/{id}?markRead=true` and `mail_message_get` `markRead: true` remain for programmatic clients. The embedded SPA must not use them to display or to select a message.
- Catalog order: after `messages.read_all`, before `messages.wait`.

## Consequences

- Cookie sessions send `X-LabMail-CSRF` on select-to-read.
- Read-only (`mail.read`) operators can inspect without flipping the bit.
- Adding a first-GA table row is a public-surface change; generate + docs/05 + docs/07 land in the same change.

## Alternatives considered

### A. SPA uses `GET ?markRead=true`

No new capability. GET is CSRF-exempt. Rejected for the inbox session.

### B. SPA calls `POST /v1/messages:read-all` on select

Marks the whole inbox. Rejected.

### C. REST-only `messages.read`

Breaks REST/MCP parity. Rejected.

## Review triggers

Review when mark-read should bump `storeGeneration`, emit audit, or when `GET ?markRead=true` is removed.
