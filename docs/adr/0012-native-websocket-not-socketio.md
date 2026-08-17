# ADR 0012: Native WebSocket instead of Socket.IO

Status: Accepted
Date: 2026-08-17

## Context

MailDev UI uses Socket.IO (`newMail`, `deleteMail`). Socket.IO is a Node-centric protocol. Go support exists but is another compatibility surface.

## Decision

Expose `GET /ws` (native WebSocket) with the same JSON event names/payloads. Patch the vendored UI client. Do not speak Socket.IO.

## Alternatives considered

- `googollee/go-socket.io` — extra protocol, polling transport, version skew.
- SSE only — workable but the UI is already event-pair oriented; WS is bidirectional-ready.
- Polling only — worse UX; MailDev users expect live update.

## Consequences

Stock MailDev 3 UI without our patch will not live-update. Our embed will.

## Compatibility impact

External Socket.IO scripts break (non-goal).

## Migration

UI patch in wave 3.

## Test impact

WS integration test; Playwright live update.

## Documentation impact

docs/13, known limitations.

## Review triggers

Need to support unmodified upstream UI without a fork.
