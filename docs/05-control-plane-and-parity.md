# Control Plane and REST/MCP Parity

Status: Proposed normative behavior
Owners: Application, REST, MCP
Last reviewed: 2026-08-17 (COMPAT-001 + MCP-001 + SEC-001)
Related ADRs: 0004, 0005, 0006, 0007

REST and MCP are two protocol adapters over one capability model. Adapters never call each other and never contain store/SMTP business logic. See [docs/adr/0004-shared-capability-registry.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0004-shared-capability-registry.md).

STA-001 implements `internal/app.Service` (HTTP-less), `internal/snapshot`, and `internal/audit`. API-001 implements `internal/capabilities` and `internal/control/rest` (`/v1`, problem+json, HMAC cursors, wait/extract, preview CSP, plan/apply). MCP-001 implements `internal/control/mcp` (Streamable HTTP `POST /mcp`, official SDK v1.7.0, protocol `2026-07-28`, `mail_*` tools, `labmail://` resources, `labmail mcp-stdio`, `make test-parity`). COMPAT-001 implements `internal/control/compat` (`/email` array list, `/healthz`, `/config`, relay 403) on the same management listener. SEC-001 implements `internal/auth` (lab static bearer, Basic→same principal, CSRF session, audit actor identity). SMTP insert stays on the data plane.

## Package layout

```
internal/capabilities     declarations (no app import)
internal/app              Service methods
internal/control/rest     HTTP /v1
internal/control/mcp      Streamable HTTP /mcp
internal/control/compat   maildev /email shim → app.Service
internal/auth             bearer + basic → Principal
internal/audit            ring
```

## Dispositions

Every public capability is one `capabilities.Capability` with REST and MCP bindings except:

| Disposition | Examples |
|---|---|
| `PARITY_REQUIRED` | messages list/get/delete/clear/wait/extract, state get/validate/export/reset, status, schema, audit, changes plan/apply |
| `REST_ONLY_PROTOCOL` | live/ready, OpenAPI, UI assets, session/CSRF, `/v1/metrics`, `/email` compat, `/healthz`, `/config`, `GET /v1/messages/{id}/preview` |
| `MCP_ONLY_PROTOCOL` | `tools/list`, `resources/list`, protocol negotiate |
| `PARITY_DIFFERENT_BINDING` | `events.stream`: REST SSE bodies vs MCP `subscriptions/listen` URI-only notify + `mail_messages_list`. Native `messages.get` default `markRead=false`; compat `GET /email/:id` marks read (maildev). |
| `EXEMPT_BY_ADR` | no OAuth PRM (ADR 0005 / TacLab 0010-equivalent) |

## Scopes

```
mail.read          list/get/wait/extract/status/schema/state get
mail.write         delete, mark-read, clear inbox
mail.admin         reset, plan/apply config, export
mail.audit.read    audit ring
```

`mail.admin` satisfies all scopes. `mail.write` does **not** include reset.

Roles (token `role` is documentation + default scope set):

| Role | Scopes |
|---|---|
| viewer | `mail.read` |
| operator | `mail.read`, `mail.write` |
| administrator | all |

## Capability table (frozen names)

| Capability ID | REST | MCP tool / resource | Scopes | Notes |
|---|---|---|---|---|
| `health.live` | `GET /v1/health/live` | — | none | REST_ONLY |
| `health.ready` | `GET /v1/health/ready` | — | none | REST_ONLY |
| `version.get` | `GET /v1/version` | `mail_version_get` | `mail.read` | |
| `capabilities.get` | `GET /v1/capabilities` | `mail_capabilities_get`, `labmail://capabilities` | `mail.read` | |
| `status.get` | `GET /v1/status` | `mail_status_get`, `labmail://status` | `mail.read` | listeners, store stats, revisions |
| `schema.get` | `GET /v1/schema/config` | `mail_schema_get`, `labmail://schema/config` | `mail.read` | |
| `state.get` | `GET /v1/state` | `mail_state_get`, `labmail://state` | `mail.read` | redacted spec + revisions |
| `state.validate` | `POST /v1/state:validate` | `mail_state_validate` | `mail.admin` | |
| `state.export` | `GET /v1/state:export` | `mail_state_export` | `mail.admin` | |
| `state.reset` | `POST /v1/state:reset` | `mail_state_reset` | `mail.admin` | wipe inbox |
| `changes.plan` | `POST /v1/changes:plan` | `mail_change_plan` | `mail.admin` | |
| `changes.apply` | `POST /v1/changes:apply` | `mail_change_apply` | `mail.admin` | expectedRevision required |
| `session.create` | `POST /v1/session` | — | none (bearer or basic) | REST_ONLY; cookie + CSRF |
| `session.delete` | `DELETE /v1/session` | — | cookie | REST_ONLY |
| `session.get` | `GET /v1/session` | — | cookie or bearer | REST_ONLY |
| `events.stream` | `GET /v1/events/stream` | MCP `subscriptions/listen` on `labmail://messages` | `mail.read` | PARITY_DIFFERENT_BINDING |
| `messages.list` | `GET /v1/messages` | `mail_messages_list`, `labmail://messages` | `mail.read` | cursor pagination |
| `messages.get` | `GET /v1/messages/{id}` | `mail_message_get`, `labmail://messages/{id}` | `mail.read` | `markRead` default **false** |
| `messages.raw` | `GET /v1/messages/{id}/raw` | `mail_message_raw_get` | `mail.read` | `message/rfc822` |
| `messages.html` | `GET /v1/messages/{id}/html` | `mail_message_html_get` | `mail.read` | raw HTML body, no CSP |
| `messages.preview` | `GET /v1/messages/{id}/preview` | — | `mail.read` | REST_ONLY; CSP document for iframe |
| `messages.delete` | `DELETE /v1/messages/{id}` | `mail_message_delete` | `mail.write` | |
| `messages.clear` | `DELETE /v1/messages` | `mail_messages_clear` | `mail.write` | |
| `messages.read_all` | `POST /v1/messages:read-all` | `mail_messages_read_all` | `mail.write` | does **not** bump `storeGeneration` |
| `messages.wait` | `POST /v1/messages:wait` | `mail_messages_wait` | `mail.read` | filter + timeout |
| `messages.extract` | `POST /v1/messages/{id}:extract` | `mail_message_extract` | `mail.read` | URLs + OTP-like tokens |
| `attachments.get` | `GET /v1/messages/{id}/attachments/{attId}` | `mail_attachment_get` | `mail.read` | |
| `audit.list` | `GET /v1/audit` | `mail_audit_query`, `labmail://audit/recent` | `mail.audit.read` | |
| `audit.get` | `GET /v1/audit/{eventId}` | `mail_audit_get` | `mail.audit.read` | |
| `metrics.get` | `GET /v1/metrics` | — | `mail.read` | REST_ONLY; only if `publicPath: true` |

`make generate` writes `api/capabilities/v1.json`, `api/openapi/v1.json`, `api/mcp/v1.json`. CI `verify-generated` fails on drift.

Renaming a tool, resource, or REST path requires an ADR plus a coordinated catalog + manifest + design-table change. MCP tool names `mail_*` are frozen.

## Parity rules

- Every public REST write operation has one or more MCP tools with equivalent semantics, except `REST_ONLY_PROTOCOL`.
- Every MCP mutation tool has a REST operation.
- REST GET representations may map to MCP resources or read tools.
- Status codes and JSON-RPC codes differ by transport, but domain error codes and error data match.
- Pagination, filtering, revisions, and authorization semantics match.
- Default values are applied in the shared application layer.
- Audit records identify the original transport but otherwise use the same event schema.
- SMTP insert stays on the data plane, not the capability registry.

## Related documents

- REST shapes: [docs/06-rest-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/06-rest-api.md)
- MCP pin: [docs/07-mcp-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/07-mcp-api.md)
- Compat `/email`: [docs/12-maildev-compat.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/12-maildev-compat.md)
