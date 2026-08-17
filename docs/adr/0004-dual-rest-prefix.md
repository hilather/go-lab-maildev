# ADR 0004: Dual REST prefix

Status: Accepted
Date: 2026-08-17

## Context

Lab smoke and labinfo use MailDev 2 paths (`/email`). MailDev 3 UI uses `/api/email`. Both must work without a lab rewrite and without forking the UI’s client more than necessary.

## Decision

Register the MailDev inspection routes twice: unprefixed (v2) and under `/api` (v3). Same handlers. Lab-native routes live under `/v1`.

## Alternatives considered

- Only `/email` and patch the UI — extra UI fork.
- Only `/api` and patch the lab first — blocks drop-in.
- Redirect `/email` → `/api/email` — breaks clients that do not follow POST/DELETE redirects.

## Consequences

Two URL spaces to document and test. Must not fork logic.

## Compatibility impact

Meets both clients.

## Migration

None.

## Test impact

Every MailDev route tested on both prefixes.

## Documentation impact

docs/06, parity plan.

## Review triggers

If MailDev 4 changes prefix again.
