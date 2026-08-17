# MailDev evaluation

Status: Normative input to the rewrite
Last reviewed: 2026-08-17
Upstream: [maildev/maildev](https://github.com/maildev/maildev)
Lab pin today: Docker image `maildev/maildev:2.2.1` in [mcp-integration-lab](https://github.com/hilather/mcp-integration-lab)

## What MailDev is

MailDev is a developer SMTP catcher: it listens for inbound SMTP, stores messages, and exposes a web UI plus REST so humans and tests can inspect mail that would otherwise have gone to a real MTA. Optional **outgoing relay** can send captured mail onward. Optional **auto-relay** forwards automatically.

It is **not** a production MTA, not an IMAP/POP server, and not a laboratory appliance with MCP/YAML/ephemeral-reset semantics in the 2.x line the lab currently runs.

Two generations matter:

| Generation | What it is | Where we saw it |
| --- | --- | --- |
| **2.2.1** | Node 18+, Express, AngularJS UI, REST at `/email`, Socket.IO, optional relay. npm `maildev@2.1.0` docs lag the 2.2.1 image. | mcp-integration-lab compose + smoke (`GET /email` with HTTP basic) |
| **3.0 (upstream `main`, 2026)** | pnpm monorepo: `@maildev/core`, `smtp`, `api` (Fastify), `ui` (React 19 + Vite + Tailwind), `mcp`, `cli`. REST moved under `/api`. MCP at `/mcp` (HTTP) plus `maildev-mcp` stdio. | Evaluated 2026-08-17 from `github.com/maildev/maildev` |

LabMail is a **parity rewrite of MailDev’s process** (SMTP catcher, REST, UI, optional relay), plus lab extras (MCP completeness, YAML, bearer). mcp-integration-lab **deploys** it receive-only.

## How mcp-integration-lab uses MailDev today

From compose, profile YAML, `internal/maildev`, and smoke:

| Plane | Default host port | Behavior |
| --- | --- | --- |
| SMTP ingest | 1025 | `net/smtp.SendMail` from smoke; no AUTH, no TLS required |
| Web UI / REST | 1080 | HTTP basic (`MAILDEV_WEB_USER` + generated password). Smoke hits **`GET /email`** (v2 path, no `/api` prefix) and expects 401 without credentials |
| MCP | none | Explicitly off-the-shelf; cataloged in labinfo only |
| Persistence | tmpfs `/tmp` | Wipe on restart/reset |
| Outbound | off in lab | `internal/maildev` rejects `outgoing-*` and `auto-relay*` before the container starts. LabMail implements those flags for comparison-lab parity. |
| Hardening | read-only rootfs, `cap_drop: ALL`, `no-new-privileges` | Healthcheck is a Node TCP connect to :1025 |

Profile `maildev/maildev.yaml` can pass other MailDev long flags (`verbose`, `hide-extensions`, `incoming-user`, …). Container-internal `--smtp` / `--web` and `--web-user` / `--web-pass` are lab-managed.

**Cutover implication:** a drop-in image must keep SMTP :1025, HTTP :1080, basic auth, `GET /email` JSON list, receive-only posture, and an unprivileged read-only container. MCP is the new lab requirement.

## MailDev 2.2.1 REST (lab contract)

Routes from `v2.2.1` `lib/routes.js` (mounted at `basePathname`, default `/`):

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/email` | All mail; `skip` pagination; extra query keys are exact-match filters with dotted paths |
| GET | `/email/:id` | Marks the message **read** |
| DELETE | `/email/:id` | |
| DELETE | `/email/all` | |
| PATCH | `/email/read-all` | Returns count |
| GET | `/email/:id/html` | HTML with CID rewritten |
| GET | `/email/:id/attachment/:filename` | |
| GET | `/email/:id/download` | `.eml` |
| GET | `/email/:id/source` | Raw RFC 5322 |
| GET | `/config` | `version`, `smtpPort`, `isOutgoingEnabled`, `outgoingHost` |
| POST | `/email/:id/relay/:relayTo?` | Outbound when configured; **ported**; comparison-lab uses `relay-sink` |
| GET | `/healthz` | JSON `true`; unauthenticated in 3.0 (2.x sits behind the same basic-auth app in typical lab use — smoke only asserts `/email` 401) |
| GET | `/reloadMailsFromDirectory` | Reload `.eml` dir |

Real-time: Socket.IO `newMail` / `deleteMail`.

JSON mail objects include `id` (8-char `[a-z0-9]`), `time`, `from`/`to` address arrays, `subject`, `text`/`html`, `headers`, `read`, `attachments`, `envelope`.

## MailDev 3.0 REST (upstream main)

Same resources under **`/api`**, plus:

| Method | Path | Notes |
| --- | --- | --- |
| POST | `/api/email/delete` | Bulk `{ "ids": [...] }` → `{ deleted, notFound }` |

Config/health live at `/api/config` and `/api/healthz`. The React UI client (`packages/ui/src/lib/api.ts`) talks only to `/api/*` and still calls **relay**.

## MailDev 3.0 MCP (upstream main)

Integrated Streamable HTTP at `/mcp` (optional `--mcp`) and stdio `maildev-mcp` that **HTTP-calls REST** (the opposite of our sibling-appliance rule).

Tools (subset of REST):

| Tool | Maps to |
| --- | --- |
| `maildev_search_emails` | Filtered list (richer than REST exact-match) |
| `maildev_get_email` | GET by id |
| `maildev_get_latest_email` | List sorted by time, limit N |
| `maildev_delete_email` | DELETE by id |
| `maildev_get_attachment` | Attachment as base64 text |

**Missing vs REST (parity gap we must close):** delete all, bulk delete, mark-all-read, HTML, source, download `.eml`, config, reload, health, **relay**. LabMail MCP includes all of those as `mail_*` tools.

Resources: `maildev://emails`, `maildev://stats`, `maildev://email/{id}`.

Prompts (MCP-only): `verify-signup-email`, `check-password-reset`, `analyze-email-content`, `monitor-email-delivery`.

Handlers format many tool results as prose text, not structured JSON. LabMail should return **structured content** (and keep a short text summary) so agents and `mcpjungle invoke` parsers can use fields.

## SMTP surface (2.x and 3.0)

Built on Nodemailer `smtp-server` + `mailparser`.

| Feature | Default / notes |
| --- | --- |
| Listen | 1025, bind `::` (any) |
| AUTH | Optional `--incoming-user` / `--incoming-pass` |
| TLS | Optional `--incoming-secure` + cert/key |
| Hide extensions | `STARTTLS`, `PIPELINING`, `8BITMIME`, `SMTPUTF8` |
| SIZE | 3.0 `--max-message-size` default 50 MiB |
| Storage | Memory; optional `--mail-directory` `.eml` + attachments |
| HTML | DOMPurify (jsdom) + CID → attachment URL rewrite |
| Relay | Optional outgoing SMTP client — **ported**; default off; lab deploy omits |

IDs: `makeId()` 8 characters from `[a-z0-9]`.

## Frontend

| Version | Stack | Keep? |
| --- | --- | --- |
| 2.x | AngularJS 1.x | No. Unmaintained, not worth embedding. |
| 3.0 | React 19, Vite, Tailwind v4, TanStack Query, Zustand, Socket.IO client | **Yes, with a small fork.** See [ADR 0005](adr/0005-vendor-maildev-3-ui.md) and [13-frontend.md](13-frontend.md). |

The 3.0 UI is a two-pane inbox: list + HTML/text/headers/source, search, keyboard shortcuts, command palette, dark mode, viewport preview, attachments, favicon badge, **Relay**. It already speaks REST. LabMail’s fork **keeps Relay** and replaces Socket.IO with `/ws` ([ADR 0012](adr/0012-native-websocket-not-socketio.md), [ADR 0013](adr/0013-full-maildev-parity-and-comparison-lab.md)).

## Node library API

`new MailDev(); maildev.listen(); maildev.on('new', …)` and middleware `basePathname` are Node embedding APIs. **Non-goal.** LabMail is a process/image, not a Go library that other apps import as an SMTP mixin. Internal packages may be imported in-repo for tests.

## What we keep (parity)

- SMTP catch-all ingest on 1025 with optional AUTH/TLS and hide-extensions.
- Captured-mail JSON shape close enough that lab smoke and the 3.0 UI work.
- REST inspection: list/filter/skip, get (marks read), delete one/all, bulk delete, mark-all-read, html, source, download, attachment, config, healthz, **relay**.
- Dual URL prefix so **v2 `/email`** (lab) and **v3 `/api/email`** (UI) hit the same handlers ([ADR 0004](adr/0004-dual-rest-prefix.md)).
- Optional mail-directory persistence (lab still mounts tmpfs).
- Web basic auth.
- HTML sanitization + CID rewrite.
- Real-time new/delete events (native WebSocket, not Socket.IO).
- MailDev 3 search/latest convenience (as first-class list filters, on **both** REST and MCP).
- MCP prompts as MCP-only.
- Outgoing relay, auto-relay, and Relay UI (proven in the comparison lab).

## What we deliberately drop

- Node programmatic API and Express middleware embedding.
- AngularJS 2.x UI **as the product UI** (still an oracle in the comparison lab).
- Socket.IO (protocol and Node adapter); UX compared via Playwright.
- Plugin SDK, webhooks, team features, databases (MailDev 3.1+ roadmap).
- Desktop `--open`.
- MCP implemented as an HTTP client to REST.

## Gaps MailDev does not fill for the lab

These are **LabMail additions**, not MailDev clones:

- MCP Streamable HTTP with **full REST parity**, bearer auth, and MCPJungle registration.
- YAML desired state (`apiVersion`) like LabDNS/LabLDAP, plus MailDev flag overlay for cutover.
- `mail_state_reset` / inbox wipe as a first-class control operation.
- `mail_email_wait` (bounded poll/wait for a matching message) for agents and tests.
- Distroless/non-root image, read-only rootfs, Go healthcheck binary (no Node in the image).
- Side-by-side comparison lab vs live MailDev (REST + UI).
- Capability registry, OpenAPI, generated MCP manifest, curated release notes, CI hardening culture.

## License interaction

MailDev is MIT. LabMail is Apache-2.0. Vendoring the 3.0 UI requires MIT attribution in `NOTICE` and keeping MailDev copyright headers on copied files. Do not relicense upstream UI files as Apache-2.0.
