# MCP API

Status: Implemented (MCP-001)
Owners: MCP, Application
Last reviewed: 2026-10-08 (reset refuses an unreadable management secret)
Related ADRs: 0004, 0006, 0008, 0009

Native management API is `/v1` + `POST /mcp`. Capability IDs and tool names are frozen in [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md). Protocol pin: [docs/adr/0006-pin-mcp-protocol-versions.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0006-pin-mcp-protocol-versions.md).

## Transport and pin

- SDK: `github.com/modelcontextprotocol/go-sdk v1.7.0`
- Protocol: `2026-07-28`
- Transport: Streamable HTTP `POST /mcp` on the management listener
- Optional: `labmail mcp-stdio --config … --token-file …` (stdout = protocol, stderr = logs). When compiled auth identity changes, mcp-stdio re-authenticates that startup secret and replaces the process actor, or drops the actor if the secret is absent or no longer matches. A reset whose management secret or password file cannot be read is refused with `validation_failed`; the previous snapshot, verifier, and startup actor stay. If that file disappears after the reset check, the previous verifier and startup actor stay.
- `Stateless: true`
- Auth: bearer only (Basic is not an MCP client convention)
- Origin check: same `auth.CheckOrigin` as REST (**missing Origin allowed**; D17 / D18). Same `originAllowlist` sentinels `"*"` / `"private"` / exact. Live-read from the snapshot. `file://` and `Origin: null` stay denied.
- MCP Streamable HTTP keeps `DisableLocalhostProtection: true`. LabMail `CheckOrigin` is the only MCP origin gate. Do not flip that SDK flag (ADR 0008 D18l).
- Direct MCP handler is POST-only (`405` after origin passes). `OPTIONS /mcp` on the management listener is REST `403` `CORS is disabled` and never reaches MCP. No CORS headers.
- Pin recorded in `internal/buildinfo` and `/v1/version`
- `spec.management.mcp.allowLegacyClients` default **false** (D17). TacLab equivalent: `api.mcp.allow_legacy_clients`. LabDNS has **no** knob (hard pin); the lab patches LabDNS. LabMail ships the TacLab knob so MCPJungle (`mark3labs/mcp-go v0.48`) can register without a LabMail patch. Integration-lab bootstrap sets `true`.
- `subscriptions/listen` stays 2026-07-28 even when the pin is relaxed.

`mail_change_plan`, `mail_change_apply`, and `mail_state_validate` accept duration and byte-size strings in operations the same way REST does (`30s`, `10MiB`). Bare numbers are `validation_failed`. Those tool schemas list `string` and `integer` so the number reaches that coercer. Other tool input schemas, and output schemas, are still inferred from the Go types. MCP structured content is the operation result **without** the HTTP problem envelope; domain `code` is always present on errors.

Resources mirror GET representations. Clients without resource support use the `mail_*` read tools.

## Tools (frozen)

| Tool | Capability | Scopes |
|---|---|---|
| `mail_version_get` | `version.get` | `mail.read` |
| `mail_capabilities_get` | `capabilities.get` | `mail.read` |
| `mail_status_get` | `status.get` | `mail.read` |
| `mail_schema_get` | `schema.get` | `mail.read` |
| `mail_state_get` | `state.get` | `mail.read` |
| `mail_state_validate` | `state.validate` | `mail.admin` |
| `mail_state_export` | `state.export` | `mail.admin` |
| `mail_state_reset` | `state.reset` | `mail.admin` |
| `mail_change_plan` | `changes.plan` | `mail.admin` |
| `mail_change_apply` | `changes.apply` | `mail.admin` |
| `mail_messages_list` | `messages.list` | `mail.read` |
| `mail_message_get` | `messages.get` | `mail.read` |
| `mail_message_raw_get` | `messages.raw` | `mail.read` |
| `mail_message_html_get` | `messages.html` | `mail.read` |
| `mail_message_delete` | `messages.delete` | `mail.write` |
| `mail_messages_clear` | `messages.clear` | `mail.write` |
| `mail_messages_read_all` | `messages.read_all` | `mail.write` |
| `mail_message_read` | `messages.read` | `mail.write` |
| `mail_messages_wait` | `messages.wait` | `mail.read` |
| `mail_message_extract` | `messages.extract` | `mail.read` |
| `mail_attachment_get` | `attachments.get` | `mail.read` |
| `mail_audit_query` | `audit.list` | `mail.audit.read` |
| `mail_audit_get` | `audit.get` | `mail.audit.read` |

`mail_change_plan`, `mail_change_apply`, and `mail_state_validate` accept the same closed operation set as REST, including `replaceSMTPBehavior` (`spec.smtp.behavior` QA handshake scripting). Operation durations and byte sizes use the same strings as REST (`30s`, `10MiB`). A bare number is `validation_failed`. The schema lists `string` and `integer` so the number reaches that coercer. There is no dedicated MCP tool per op.

Health live/ready, OpenAPI, UI assets, session/CSRF, `/v1/metrics`, `/email` compat, `/healthz`, `/config`, and `GET /v1/messages/{id}/preview` are **not** MCP tools.

## Resources (frozen)

| URI | Capability |
|---|---|
| `labmail://capabilities` | `capabilities.get` |
| `labmail://status` | `status.get` |
| `labmail://schema/config` | `schema.get` |
| `labmail://state` | `state.get` |
| `labmail://messages` | `messages.list` |
| `labmail://messages/{id}` | `messages.get` |
| `labmail://audit/recent` | `audit.list` |

`subscriptions/listen` on `labmail://messages` notifies **URI only**; clients pull bodies with `mail_messages_list`. Inbox `notifications/resources/updated` is emitted through the official SDK `ResourceUpdated` path so Streamable HTTP and `labmail mcp-stdio` share one notify implementation. The HTTP adapter intercepts `subscriptions/listen` only to enforce the D17 protocol pin (`2026-07-28` on the header or `_meta`).

## Auth

MCP is bearer-only. Basic is not accepted on `/mcp`. Tokens are the same lab static bearer set as REST (`spec.management.auth.tokens`). No OAuth Protected Resource Metadata.

## Compatibility promise

MCP tool names `mail_*` are frozen; rename needs ADR + catalog change.
