# Control plane and REST/MCP parity

Status: Proposed normative
Last reviewed: 2026-08-17
Related ADRs: 0003, 0004, 0006, 0011

## Problem statement

MailDev 3’s MCP is a convenience subset and, in stdio mode, an HTTP client of REST. Sibling lab appliances treat REST and MCP as two adapters over one registry. LabMail follows the sibling rule: **no capability exists on only one agent-facing transport** unless it is protocol infrastructure.

## Capability record

Each public capability is declared once in `internal/capabilities` (no `Handler` func; adapters bind by `ServiceMethods` name so this package does not import `app`).

```text
ID, Title, Version, Description
InputSchema, OutputSchema
RequiredScopes
Mutating, Idempotent
Disposition: PARITY_REQUIRED | REST_ONLY_PROTOCOL | MCP_ONLY_PROTOCOL
REST bindings (method + path templates, including dual-prefix aliases)
MCP bindings (tool name and/or resource URI)
ServiceMethods []string
```

The registry generates OpenAPI metadata, MCP manifests, docs tables, and parity tests.

## Frozen capability table (v1)

Scopes: `mail.read`, `mail.write`, `mail.admin`. Health probes: none.

| ID | REST | MCP | Scopes | Disposition |
| --- | --- | --- | --- | --- |
| `health.live` | `GET /v1/health/live` | — | — | REST_ONLY_PROTOCOL |
| `health.ready` | `GET /v1/health/ready` | — | — | REST_ONLY_PROTOCOL |
| `health.healthz` | `GET /healthz`, `GET /api/healthz` | — | — | REST_ONLY_PROTOCOL |
| `version.get` | `GET /v1/version` | `mail_version_get` | mail.read | PARITY_REQUIRED |
| `capabilities.get` | `GET /v1/capabilities` | `mail_capabilities_get`, `labmail://capabilities` | mail.read | PARITY_REQUIRED |
| `status.get` | `GET /v1/status` | `mail_status_get`, `labmail://status` | mail.read | PARITY_REQUIRED |
| `config.get` | `GET /config`, `GET /api/config`, `GET /v1/config` | `mail_config_get`, `labmail://config` | mail.read | PARITY_REQUIRED |
| `stats.get` | `GET /v1/stats` | `mail_stats_get`, `labmail://stats` | mail.read | PARITY_REQUIRED |
| `emails.list` | `GET /email`, `GET /api/email` | `mail_emails_list`, `labmail://emails` | mail.read | PARITY_REQUIRED |
| `emails.search` | `GET /v1/emails:search` (also list query params) | `mail_emails_search` | mail.read | PARITY_REQUIRED |
| `emails.latest` | `GET /v1/emails:latest` | `mail_email_get_latest` | mail.read | PARITY_REQUIRED |
| `emails.wait` | `POST /v1/emails:wait` | `mail_email_wait` | mail.read | PARITY_REQUIRED |
| `email.get` | `GET /email/{id}`, `GET /api/email/{id}` | `mail_email_get`, `labmail://emails/{id}` | mail.read | PARITY_REQUIRED |
| `email.delete` | `DELETE /email/{id}`, `DELETE /api/email/{id}` | `mail_email_delete` | mail.write | PARITY_REQUIRED |
| `emails.delete_bulk` | `POST /email/delete`, `POST /api/email/delete` | `mail_emails_delete` | mail.write | PARITY_REQUIRED |
| `emails.delete_all` | `DELETE /email/all`, `DELETE /api/email/all` | `mail_emails_delete_all` | mail.write | PARITY_REQUIRED |
| `emails.read_all` | `PATCH /email/read-all`, `PATCH /api/email/read-all` | `mail_emails_mark_read_all` | mail.write | PARITY_REQUIRED |
| `email.html` | `GET /email/{id}/html`, `GET /api/email/{id}/html` | `mail_email_html_get` | mail.read | PARITY_REQUIRED |
| `email.source` | `GET /email/{id}/source`, `GET /api/email/{id}/source` | `mail_email_source_get` | mail.read | PARITY_REQUIRED |
| `email.download` | `GET /email/{id}/download`, `GET /api/email/{id}/download` | `mail_email_download` | mail.read | PARITY_REQUIRED |
| `email.attachment` | `GET /email/{id}/attachment/{filename}` (+ `/api`) | `mail_attachment_get` | mail.read | PARITY_REQUIRED |
| `store.reload` | `GET /reloadMailsFromDirectory` (+ `/api`) | `mail_store_reload` | mail.admin | PARITY_REQUIRED |
| `state.reset` | `POST /v1/state:reset` | `mail_state_reset` | mail.admin | PARITY_REQUIRED |
| `schema.get` | `GET /v1/schema/config` | `mail_schema_get`, `labmail://schema/config` | mail.read | PARITY_REQUIRED |
| `prompt.verify_signup` | — | prompt `verify-signup-email` | mail.read | MCP_ONLY_PROTOCOL |
| `prompt.password_reset` | — | prompt `check-password-reset` | mail.read | MCP_ONLY_PROTOCOL |
| `prompt.analyze` | — | prompt `analyze-email-content` | mail.read | MCP_ONLY_PROTOCOL |
| `prompt.monitor` | — | prompt `monitor-email-delivery` | mail.read | MCP_ONLY_PROTOCOL |
| `openapi.get` | `GET /openapi.json` | — | mail.read | REST_ONLY_PROTOCOL |
| `ui.spa` | `GET /`, static | — | (same HTTP auth) | REST_ONLY_PROTOCOL |
| `events.ws` | `GET /ws` | — | mail.read | REST_ONLY_PROTOCOL |

WebSocket is REST_ONLY_PROTOCOL (framing). MCP may later add `notifications` / subscriptions via ADR; not required for GA if `mail_email_wait` covers agent blocking.

**Forbidden IDs:** `email.relay`, `smtp.send`, anything that delivers mail.

## Side effects that must match

| Operation | Side effect |
| --- | --- |
| `email.get` | Marks read (`true`) unless both transports grow an explicit flag together |
| `email.delete` / bulk / all | Emits `MailDeleted` / reset events |
| `emails.read_all` | Returns count of newly marked |
| `state.reset` | Inbox empty; config unchanged |
| `emails.wait` | Does **not** mark read until a subsequent get |

## Binary payloads on MCP

HTML, source, download, attachment: MCP returns structured JSON with `mediaType` and `base64` (and `byteLength`). REST returns native content types. Parity tests compare decoded bytes.

Cap MCP attachment/download size (default 5 MiB encoded); over-size: `payload_too_large` with REST still able to stream. Document in known limitations if the cap is lower than REST.

## Errors

Domain codes from [17-error-model.md](17-error-model.md). REST: `application/problem+json` for `/v1/*`; MailDev-shaped `{ "error": "Email was not found" }` **also** on `/email` and `/api/email` 404s for UI compatibility. MCP: JSON-RPC error `data.code` equals the domain code.

## Authorization

| Scope | Typical tools |
| --- | --- |
| `mail.read` | list, get, search, wait, html, source, download, attachment, stats, config, version |
| `mail.write` | delete, bulk delete, delete all, mark read all |
| `mail.admin` | reset, reload |

When only HTTP basic is configured (lab MailDev mode), successful basic auth grants **all three scopes** (MailDev has no RBAC). When bearer tokens exist, each token has scopes. A token with only `mail.read` cannot delete.

## Mutation metadata (lab-grade, light)

Unlike LabDNS plan/apply, inbox mutations are direct. Still record:

- actor (basic user or token id, never secret)
- transport (`rest` \| `mcp` \| `smtp` \| `ws`)
- reason optional on reset

No optimistic concurrency on emails (ids are unique; delete missing = 404). Reset is idempotent.

## Parity verification

`make test-parity` must:

1. Load the registry.
2. Fail if a `PARITY_REQUIRED` row lacks REST or MCP bindings.
3. For each mutating and read tool, run a fixture against REST and MCP and compare domain JSON (bytes for binaries).

Wave 3 owns the harness; wave 1 owns the empty registry test.
