# System architecture

Status: Proposed normative
Owners: Architecture, SMTP, Control plane
Last reviewed: 2026-08-17
Related ADRs: 0001–0012

## Problem statement

mcp-integration-lab needs a receive-only SMTP sink that systems under test can send to, that agents can inspect through the same MCP gateway as DNS/LDAP/TACACS, and that operators can open in a browser. Off-the-shelf MailDev 2.2.1 covers SMTP + REST + UI but has no MCP, is a Node image, and its relay flags have to be externally banned. MailDev 3.0 adds a partial MCP surface that still is not REST-complete and still can send mail.

LabMail is a single Go process that keeps the MailDev inspection experience (including the 3.0 UI) while matching sibling lab appliances: YAML desired state, ephemeral runtime, REST/MCP parity, bearer-friendly MCP, unprivileged containers.

## Goals

- Accept SMTP from real clients used in integration tests (Go `net/smtp`, Python `smtplib`, Nodemailer, JavaMail, swaks).
- Never send or relay mail.
- Expose every public REST control capability on MCP (and the reverse, except documented protocol-only rows).
- Serve the MailDev 3 UI from the same management listener.
- Drop-in for lab ports 1025/1080, basic auth, and `GET /email`.
- Wipe captured mail on restart and on reset.
- Run as non-root, read-only rootfs, `cap_drop: ALL`.

## Non-goals

- Outbound SMTP, smarthost, DKIM/SPF signing, greylisting, queues.
- IMAP, POP3, JMAP, milter.
- Multi-replica shared inbox.
- Node embedding API.
- Implementing MCP by looping through REST.

## Invariants

1. SMTP ingest does not depend on REST or MCP availability.
2. No code path opens a client connection to deliver mail.
3. REST, MCP, WebSocket, and UI call `internal/app` only.
4. Captured mail is not written to bootstrap YAML.
5. Unknown configuration fields are errors.
6. Relay/outbound keys and MailDev relay flags fail closed at config compile.
7. Management auth is required except documented probes (`/healthz`, `/v1/health/*`).
8. HTML preview is sanitized before it is stored for UI/HTML routes.
9. Message size and store cardinality are bounded.
10. Dual REST prefixes are aliases, not two implementations.

## Context diagram

```text
  systems under test                operators / browsers
           |                                  |
           | SMTP :1025                       | HTTP :1080
           v                                  v
     +-----------+                     +-------------+
     |  smtpd    | --parsed mail-----> |    store    |
     +-----------+                     |  (memory)   |
                                       +------+------+
                                              |
                         +--------------------+--------------------+
                         |                    |                    |
                   REST adapter          MCP adapter         WS adapter
                   /email, /api          /mcp                /ws
                         |                    |                    |
                         +-------- internal/app --------+          |
                                              |                    |
                                         MailDev 3 SPA <-----------+
```

mcp-integration-lab registers `/mcp` in MCPJungle with a bearer token. Labinfo catalogs SMTP and the UI/REST URLs.

## Process model

One binary `labmaild`:

| Listener | Default container port | Role |
| --- | --- | --- |
| SMTP | 1025 | Data plane ingest |
| HTTP | 1080 | UI, REST (both prefixes), `/mcp`, `/ws`, health, OpenAPI |

Optional: SMTP implicit TLS on a separate port if `incoming-secure` is set; STARTTLS on 1025 when configured and not hidden.

No writable persistent volume required. Optional mail-directory is a tmpfs or explicit operator mount.

## Package boundaries

```text
cmd/labmaild                 process + cobra/flag wiring
internal/model               Email, Attachment, Envelope, Config (canonical)
internal/app                 operations used by all adapters
internal/config              YAML + MailDev flags + env; reject relay
internal/store               bounded in-memory inbox (+ optional dir)
internal/smtpd               go-smtp adapter; no app/REST imports
internal/mime                parse RFC 5322, decode, split parts
internal/sanitize            HTML policy adapter (bluemonday or equivalent)
internal/control/rest        HTTP routes, dual prefix
internal/control/mcp         Streamable HTTP + optional stdio
internal/control/ws          native WebSocket new/delete
internal/capabilities        registry metadata (no app import)
internal/auth                basic + bearer
internal/domainerr           stable error codes
internal/web                 embed built SPA
internal/observability       slog, metrics, health
```

Forbidden imports:

- `control/*` ← must not import each other.
- `smtpd` ← must not import `control`, `web`, or MCP.
- `capabilities` ← must not import `app`.
- `store` ← must not import HTTP or MCP.

## Data flow: ingest

1. SMTP session accepted (optional AUTH).
2. Size checked (SIZE extension + hard cap).
3. DATA read into a bounded buffer.
4. MIME parse → domain `Email` + attachment blobs.
5. HTML sanitized; CID map recorded.
6. Store assign 8-char id, `time=now`, `read=false`.
7. Emit `MailReceived` to WS subscribers and audit/metrics.

SMTP success is independent of whether anyone is watching REST/MCP/UI.

## Data flow: control

1. Adapter authenticates (basic or bearer).
2. Adapter authorizes scope (`mail.read` / `mail.write` / `mail.admin`).
3. Adapter decodes transport → domain request.
4. `internal/app` executes against the store.
5. Domain response → JSON (REST), structured MCP content, or WS event.

GET-by-id marks read in `app`, not in the REST layer, so MCP get has the same side effect unless a `markRead=false` flag is explicitly added to **both** transports together.

## Configuration layers

Priority (highest wins), matching MailDev 3 merge order as closely as the overlay allows:

1. CLI flags (MailDev names + LabMail names)
2. Process environment (`MAILDEV_*` and `LABMAIL_*`)
3. YAML file (`--config` / default path)
4. Built-in defaults

See [04-state-and-configuration.md](04-state-and-configuration.md) and [ADR 0010](adr/0010-yaml-plus-maildev-flags.md).

## Identity and naming

| Kind | Value |
| --- | --- |
| Repository | `github.com/hilather/go-lab-maildev` |
| Product | LabMail |
| Binary / image command | `labmaild` |
| Image name (planned) | `ghcr.io/hilather/go-lab-maildev` |
| MCP server name | `labmail` |
| Tool prefix | `mail_*` |
| Resource URI prefix | `labmail://` |
| Config apiVersion | `labmail.dev/v1alpha1` |

## Relationship to sibling appliances

| Concern | LabDNS / TacLab / LabLDAP | LabMail |
| --- | --- | --- |
| Desired state | YAML bootstrap | YAML listen/auth/limits |
| Runtime overlay | DNS/AAA objects | Captured messages |
| Reset | Restore YAML snapshot | Wipe inbox (config stays) |
| MCP | Streamable HTTP `/mcp` | Same |
| Auth | Bearer (typical) | Bearer **and** MailDev basic |
| UI | First-party React | Vendored MailDev 3 React |

## First-GA container posture

Match the lab maildev service as closely as a Go image can:

- Non-root UID (65532 or similar)
- Read-only rootfs
- `cap_drop: ALL`
- `no-new-privileges`
- tmpfs for `/tmp` (and optional mail-directory)
- Healthcheck: `labmaild healthcheck` against SMTP and HTTP ready
- No Node runtime in the image
