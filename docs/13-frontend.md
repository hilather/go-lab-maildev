# Frontend (MailDev 3 UI)

Status: Proposed normative
Last reviewed: 2026-08-17
Related ADR: [0005](adr/0005-vendor-maildev-3-ui.md), [0012](adr/0012-native-websocket-not-socketio.md)

## Decision

**Keep the MailDev 3.0 React UI.** It already implements the operator inbox we want. Rebuilding it would duplicate a large, working SPA. The 2.x AngularJS UI is not kept.

Vendor under `web/` with an explicit delta list and MIT attribution in `NOTICE`.

## Why this is appropriate

- Talks REST (`/api/email`, html, source, download, attachments, config, healthz).
- LabMail will dual-mount those routes.
- Features: two-pane layout, search, HTML/text/headers/source, viewport sizes, shortcuts, command palette, dark mode, unread badge.
- Sibling appliances also embed React SPAs (TacLab, LabLDAP).

## Required fork delta (GA)

1. **Replace Socket.IO** with native `WebSocket` to `${basePath}/ws` (JSON `newMail` / `deleteMail`). Remove `socket.io-client`.
2. **Remove Relay** from `api.ts`, command palette, email header buttons, and types.
3. Treat `config.isOutgoingEnabled` as always false; do not show relay settings.
4. Keep using `/api` as the client prefix (server compatibility is the dual-mount).
5. Do not persist bearer tokens in `localStorage`. Basic auth is handled by the browser for same-origin UI. If we add a token login later, memory-only (ADR).
6. HTML preview iframe: existing sandbox discipline stays; HTML is already sanitized server-side.

## Build

- Node 22 + lockfile in `web/`.
- `make web-build` → `web/dist`.
- Embed via `//go:embed` in `internal/web`.
- SPA fallback: unknown non-API paths serve `index.html` except `/api`, `/email`, `/mcp`, `/ws`, `/v1`, `/healthz`.

## Operator workflows that must work (Playwright)

- See new mail appear without full page reload (WS).
- Open HTML, text, headers, source tabs.
- Download `.eml` and an attachment.
- Delete one, delete all, mark all read.
- Search/filter.
- Unauthenticated visit → browser basic-auth prompt or 401 page (not a blank SPA error storm).

## What the UI must not do

- Call relay endpoints.
- Talk to MCP.
- Duplicate sanitizer logic (display server HTML).
- Require Node at runtime.

## Upstream tracking

Record the MailDev commit vendored in `web/UPSTREAM.md`. Do not silently merge upstream relay or Socket.IO back in. Prefer cherry-picks of inbox bugfixes.
