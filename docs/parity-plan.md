# Parity plan

Status: Normative for implementation waves
Last reviewed: 2026-08-17

LabMail parity has **four axes**. A MailDev capability is not done until Axis A (and D) pass. Lab extras still need Axis C.

```text
Axis A  MailDev process behavior (2.2.1 + 3.0) — every feature ported
Axis B  mcp-integration-lab drop-in + first-class MCP
Axis C  REST ↔ MCP (and UI where it is an operator workflow)
Axis D  Side-by-side Docker comparison lab (REST + UI vs live MailDev)
```

Outgoing/auto-relay **are** on Axis A and D. They stay **off** in Axis B deployments ([ADR 0013](adr/0013-full-maildev-parity-and-comparison-lab.md)).

## Axis A — MailDev inspection

### A1. SMTP ingest (wave 2)

| MailDev | LabMail | Tests |
| --- | --- | --- |
| Port 1025, bind any | Same defaults | Dial + DATA |
| Optional AUTH | PLAIN/LOGIN | Auth success/fail |
| Optional incoming TLS | STARTTLS and/or implicit TLS | Handshake |
| `--hide-extensions` | Same names | EHLO body |
| SIZE / max message | Default 50 MiB | 552/552-equivalent + no store |
| Catch-all RCPT | Accept any recipient | Multiple RCPT |
| Envelope host/remote | Stored on record | JSON field |
| 8BITMIME / SMTPUTF8 | Advertise unless hidden | UTF-8 subject/body |

### A2. Stored mail model (wave 2)

Preserve fields the UI and lab smoke need: `id`, `time`, `read`, `subject`, `from`, `to`, `cc`, `bcc`, `calculatedBcc`, `date`, `text`, `html`, `headers`, `priority`, `attachments[]`, `envelope`, `size`, `sizeHuman`. Raw source available. Attachment `contentType`, `fileName`/`filename`, `generatedFileName`, `contentId`, `contentDisposition`, `size`.

Compatibility: accept both `fileName` (v2 samples) and `filename` (v3 types) in tests; **emit `filename` and `generatedFileName`** as in 3.0, plus `fileName` duplicate if a v2 client is proven to need it. Decide in wave 2 with a golden JSON fixture from a 2.2.1 capture; document in [03-mail-model.md](03-mail-model.md).

### A3. REST (wave 3)

Implement the union of 2.2.1 and 3.0 inspection routes, on **both** prefixes ([ADR 0004](adr/0004-dual-rest-prefix.md)):

| Capability | v2 path | v3 path |
| --- | --- | --- |
| List + skip + dotted filters | `GET /email` | `GET /api/email` |
| Get (marks read) | `GET /email/:id` | `GET /api/email/:id` |
| Delete one | `DELETE /email/:id` | `DELETE /api/email/:id` |
| Delete all | `DELETE /email/all` | `DELETE /api/email/all` |
| Bulk delete | *(3.0 only)* | `POST /api/email/delete` **and** `POST /email/delete` |
| Mark all read | `PATCH /email/read-all` | `PATCH /api/email/read-all` |
| HTML | `GET /email/:id/html` | `GET /api/email/:id/html` |
| Source | `GET /email/:id/source` | `GET /api/email/:id/source` |
| Download | `GET /email/:id/download` | `GET /api/email/:id/download` |
| Attachment | `GET /email/:id/attachment/:filename` | same under `/api` |
| Config | `GET /config` | `GET /api/config` |
| Health | `GET /healthz` | `GET /api/healthz` |
| Reload dir | `GET /reloadMailsFromDirectory` | `GET /api/reloadMailsFromDirectory` |
| Relay | `POST /email/:id/relay/:relayTo?` | `POST /api/email/:id/relay/:relayTo?` |

**Relay** is implemented ([ADR 0013](adr/0013-full-maildev-parity-and-comparison-lab.md)). When outgoing is off, match MailDev’s error (500 “Outgoing mail not configured” class). When on, deliver to the configured host (comparison lab: `relay-sink`).

**Added REST** (lab, must also be MCP):

| Capability | REST |
| --- | --- |
| Search (MailDev MCP filters) | `GET /v1/emails:search` (and query on list) |
| Latest N | `GET /v1/emails:latest?count=` |
| Wait | `POST /v1/emails:wait` |
| Stats | `GET /v1/stats` |
| Reset inbox | `POST /v1/state:reset` |
| Version / capabilities | `GET /v1/version`, `GET /v1/capabilities` |

List search filters (`from`, `to`, `subject`, `query`, `hasAttachment`, `isUnread`, `since`, `until`, `limit`) are available on **both** `/email` (as extra query params; `skip` remains) and `/v1` so MCP search is not MCP-only.

### A4. UI (wave 3)

Vendor MailDev 3 UI. Operator workflows that exist as buttons today (list, open, delete one, delete all, mark all read, download, attachments, HTML/text/headers/source, search, refresh, **Relay** when outgoing is on) must work. Live updates via `/ws`. Comparison-lab Playwright drives **both** the original UIs and LabMail ([23-behavior-parity-matrix.md](23-behavior-parity-matrix.md)).

### A5. Real-time (wave 3)

MailDev Socket.IO events `newMail` and `deleteMail`. LabMail native WebSocket JSON:

```json
{ "type": "newMail", "email": { ... } }
{ "type": "deleteMail", "data": { "id": "..." } }
```

Payload `email` uses the same JSON as REST get (without forcing mark-read on subscribers).

## Axis B — Lab drop-in and MCPJungle

| Lab contract today | LabMail |
| --- | --- |
| Image `maildev/maildev:2.2.1` | `ghcr.io/hilather/go-lab-maildev:<tag>` built in-lab like LabDNS |
| `command: ${MAILDEV_ARGS}` | `labmaild serve ${MAILDEV_ARGS}` with same flag renderer |
| `MAILDEV_WEB_USER` / `MAILDEV_WEB_PASS` | Honored |
| Smoke: SMTP + `GET /email` 401 + authed list contains subject | Must pass unchanged |
| labinfo URLs `/` and `/email` | Keep; add MCP URL when lab registers it |
| No MCPJungle server JSON | Add `labmail.json` streamable HTTP + bearer |
| Relay flags rejected in `internal/maildev` | **Keep that rejector in the lab repo.** LabMail **accepts** the flags for MailDev parity; integration-lab profiles must not set them |
| tmpfs, read-only, cap_drop | Same |
| Healthcheck Node TCP 1025 | `labmaild healthcheck` |

Cutover steps live in [12-lab-integration.md](12-lab-integration.md). That work is a **follow-up PR on mcp-integration-lab**, not this repo, but LabMail must not require lab changes beyond image/command/token/MCP registration.

## Axis C — REST / MCP parity

Frozen names: [05-control-plane-and-parity.md](05-control-plane-and-parity.md).

Rules:

1. If it is on REST and is not `REST_ONLY_PROTOCOL`, it has an MCP tool and/or resource in the **same change**.
2. If it is an MCP tool and is not `MCP_ONLY_PROTOCOL`, it has REST.
3. Same `internal/app` method, scopes, validation, mark-read side effect, redaction.
4. Parity tests compare domain results, not status-code strings.

MailDev 3’s five tools become LabMail tools **and** REST. Prompts stay MCP-only.

### Convenience vs primitives

`mail_emails_search` and `mail_email_get_latest` are not “extra MCP magic”; they are parameterized list operations with REST equivalents. `mail_email_wait` is a lab addition on both transports (timeout bounded, cancellable).

## Axis D — Comparison lab

Normative: [22-comparison-lab.md](22-comparison-lab.md), [23-behavior-parity-matrix.md](23-behavior-parity-matrix.md), wave CMP.

- Oracles: MailDev 2.2.1 + MailDev 3.0 (pinned) + LabMail + `relay-sink`.
- Every `req` REST and UI cell must pass.
- Characterization against MailDev can start before LabMail exists.

## Sequencing

```text
Wave 0    contracts (this pack)
Wave CMP  oracle compose + REST/UI harness (parallel with 1–3)
Wave 1    module, schema, capability IDs frozen in code
Wave 2    SMTP + MIME + store + relay client
Wave 3    app + REST + MCP + WS + UI (Relay kept)
Wave 4    image + compose + lab cutover doc + in-process parity
Wave 5    interop clients, release notes automation, GA (CMP green required)
```

## Definition of MailDev parity for GA

GA **does** mean every MailDev **process** feature in the [behavior matrix](23-behavior-parity-matrix.md) is ported and comparison-lab green:

- Lab smoke would pass against LabMail with the current `GET /email` assertions.
- MailDev 3 UI (fork) can inspect, search, delete, preview, download, and **relay** when outgoing is configured.
- Side-by-side REST + UI tests vs live MailDev v2 and v3 pass.
- Every LabMail REST control capability is callable via MCP with matching structured results.
- mcp-integration-lab deploy still omits outgoing.
- Container matches lab security posture.

## Explicit non-parity

| MailDev | Disposition |
| --- | --- |
| Node `require('maildev')` | Out of scope |
| AngularJS UI **as LabMail’s UI** | Out of scope (v2 UI is still an **oracle** in the comparison lab) |
| Socket.IO protocol | Replaced by `/ws`; UX compared in the lab |
| `maildev init` wizard | Optional later; YAML examples suffice |
| Homebrew / npm publish | Out of scope |
| Plugin SDK | Out of scope |
