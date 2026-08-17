# maildev Compatibility Surface

Status: Proposed normative behavior
Owners: Compat, REST, Application
Last reviewed: 2026-08-17 (FND-001)
Related ADRs: 0005, 0007

Native management API is `/v1` + `POST /mcp`. Maildev `/email` is a **compat adapter** (`REST_ONLY_PROTOCOL` plus parity-required native twins). See [docs/adr/0007-compat-email-surface.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0007-compat-email-surface.md).

Enabled when `spec.listeners.management.compatEnabled` is true (default). Auth: same middleware as `/v1` (Basic and/or Bearer).

The lab does **not** track maildev v3 (`/api`, optional MCP). LabMail’s native API is the family `/v1` + `/mcp` design.

## Path mapping

| maildev 2.2.1 | LabMail compat | Mapping |
|---|---|---|
| `GET /email` | same | `Messages.List` as a JSON **array**. `?skip=` supported. Filter: exact `subject=` plus maildev-style dotted keys (`headers.to=`) as exact string match on flattened fields. |
| `GET /email/:id` | same | `Messages.Get` with `markRead=true` (maildev) |
| `DELETE /email/:id` | same | `Messages.Delete` |
| `DELETE /email/all` | same | `Messages.Clear` |
| `GET /email/:id/html` | same | HTML body; same CSP headers as `/v1/.../preview` when used as a document |
| `GET /email/:id/attachment/:filename` | same | lookup by sanitized filename; first match |
| `POST /email/:id/relay` | same path | **403** `receive_only`. Body explains receive-only. No-op must not look like success. |
| `GET /config` | same | Redacted LabMail shape (below). **Not** a clone of 2.2.1 `/config`. |
| `GET /healthz` | same | 200 `{"status":"ok"}` iff ready |

## Compat email JSON

Smoke needs `subject`; keep the 2.2.1 shape:

```json
{
  "id": "01JEXAMPLENOTAREALULID0001",
  "time": "2026-08-17T12:00:00.000Z",
  "from": [{ "address": "smoke@lab.test", "name": "" }],
  "to":   [{ "address": "inbox@lab.test", "name": "" }],
  "cc": [],
  "bcc": [],
  "subject": "mcplab smoke 1",
  "text": "",
  "html": "",
  "headers": { "from": "…", "to": "…", "subject": "…" },
  "read": false,
  "messageId": "ulid@labmail.lab",
  "priority": "normal",
  "attachments": [],
  "envelope": {
    "from": "smoke@lab.test",
    "to": ["inbox@lab.test"],
    "host": "client.example",
    "remoteAddress": "10.42.0.9"
  }
}
```

`headers` values are strings. Header **map keys are lowercased** (maildev 2.2.1). Duplicate headers are joined with `\n`. Attachment objects **omit** maildev’s leaked `stream` Node internals; they include `fileName`, `contentType`, `contentDisposition`, `contentId`, `checksum` (**sha256** hex — maildev 2.2.1 uses md5; documented delta).

`GET /email` **list** omits `html` and sets `text` to `""` (or a ≤2 KiB prefix if `?text=1`). Smoke only needs `subject`. This is an intentional maildev delta so a 1000-message inbox cannot serialize hundreds of MiB. `GET /email/:id` returns full `text`/`html`.

`GET /email` returns **all** matching messages (maildev style) up to `store.maxMessages`. Native `/v1` is the paginated API.

`GET /config` is a redacted LabMail shape: `{smtp, web, receiveOnly: true, hostname}` only. Always `receiveOnly: true`.

## Compat delta vs maildev 2.2.1

Goldens from `maildev/maildev:2.2.1` for `subject`/`from`/`to`/`id`/`time`/`read` plus one attachment fixture with `fileName` and **no** `stream`:

| Field / behavior | maildev 2.2.1 | LabMail compat |
|---|---|---|
| `id` | 8-char | ULID 26-char |
| `messageId` | bare (no `<>`) | bare (no `<>`) |
| header keys | lowercased | lowercased |
| list `text`/`html` | full bodies | omitted (`""`) |
| attachment `checksum` | md5 | sha256 |
| attachment `stream` | leaked Node object | omitted |
| filter | any field + dotted | same: exact match, dotted `headers.*` |
| `GET /config` | maildev internals | `{smtp, web, receiveOnly: true, hostname}` only |
| `POST /email/:id/relay` | sends if configured | always 403 |
| WebSocket | yes | not implemented |

## maildev CLI flags we do not take as a schema

YAML replaces them. Semantic coverage:

| maildev flag | LabMail 1.0 |
|---|---|
| `--smtp` / `--web` | YAML listeners; container ports fixed at 1025 / 1080 |
| `--web-user` / `--web-pass` | `spec.management.auth.basic` file refs (compat) |
| `--incoming-user` / `--incoming-pass` | `spec.smtp.auth` file refs |
| `--incoming-secure` + cert/key | **Rejected in 1.0.** maildev means implicit SMTPS (TLS-on-accept), not STARTTLS. Do not silently map. |
| `--hide-extensions` | `spec.smtp.hideExtensions` |
| `--ip` / `--web-ip` | listener `address` |
| `--mail-directory` | **Rejected.** Store is memory; optional tmpfs spill is not durable. |
| `--outgoing-*`, `--auto-relay*` | **Schema-impossible.** Unknown-field / reserved-name reject |
| `--https` (web) | `spec.listeners.management.tls` (optional; lab stays HTTP) |
| `--disable-web` | `spec.ui.enabled: false` |
| `--base-pathname` | Non-goal |
| `--mcp` (upstream v3) | Always-on Streamable HTTP at `/mcp` when management is bound |
| WebSocket live inbox | Non-goal. Family uses SSE + MCP subscribe |

The one-release `internal/maildev` flag shim matrix lives in [docs/13-integration-lab-swap.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/13-integration-lab-swap.md).

## Acceptance test the swap must keep green

`mcp-integration-lab` `internal/lab/smoke.go` `maildevScenario`:

1. `net/smtp.SendMail` to `127.0.0.1:${MAILDEV_SMTP_PORT}` with no auth.
2. Unauthenticated `GET /email` → **401**.
3. Basic-authenticated `GET /email` eventually contains the sent `subject`.

In this repo: `TestMaildevScenarioCompat` (PR 9). PR 7 implements the adapter with a fake principal and does **not** claim 401.

## Compatibility promise

Compat `/email` is best-effort maildev 2.2.1 **read/delete** shape; relay will never work.
