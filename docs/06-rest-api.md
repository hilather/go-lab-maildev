# REST API

Status: Proposed normative
Last reviewed: 2026-08-17
Related: [05-control-plane-and-parity.md](05-control-plane-and-parity.md), [ADR 0004](adr/0004-dual-rest-prefix.md)

OpenAPI will be generated from the capability registry + `internal/model` in wave 3 (`api/openapi/v1.yaml`). Until then this document is the contract.

## Listeners and prefixes

Management HTTP (default `:1080`):

| Prefix | Audience |
| --- | --- |
| `/email`, `/config`, `/healthz`, `/reloadMailsFromDirectory` | MailDev 2.2.1 / mcp-integration-lab smoke |
| `/api/email`, `/api/config`, `/api/healthz`, … | MailDev 3 UI |
| `/v1/*` | Lab-native, OpenAPI-first |
| `/mcp` | MCP |
| `/ws` | UI events |
| `/` | SPA |

`--base-pathname /maildev` prefixes **all** of the above (MailDev behavior).

## Authentication

| Route | Auth |
| --- | --- |
| `/v1/health/live`, `/v1/health/ready` | none |
| `/healthz`, `/api/healthz` | none (MailDev 3); documented |
| everything else | HTTP basic and/or `Authorization: Bearer` |

401 + `WWW-Authenticate: Basic realm="LabMail"` when basic is enabled and no/invalid credentials. Bearer-only deployments may omit WWW-Authenticate basic.

## MailDev-compatible JSON

List: JSON array of email objects ([03-mail-model.md](03-mail-model.md)).

Get missing: `404 { "error": "Email was not found" }`.

Delete success: JSON `true` (MailDev). Bulk delete: `{ "deleted": [...], "notFound": [...] }`.

Read-all: JSON number (count).

Healthz: JSON `true`.

Config:

```json
{
  "version": "0.0.0-dev",
  "smtpPort": 1025,
  "isOutgoingEnabled": false,
  "outgoingHost": null
}
```

`isOutgoingEnabled` / `outgoingHost` reflect config (MailDev). Default false/null. Comparison-lab `relay` profile sets them.

## Filtering and pagination

`GET /email?skip=10&subject=test&from.address=a@b.c`

Reserved query keys: `skip`, `limit`, `query`, `hasAttachment`, `isUnread`, `since`, `until`, `body`. All other keys are dotted exact-match filters.

## `/v1` additions

| Method | Path | Body / query |
| --- | --- | --- |
| GET | `/v1/emails:search` | search fields |
| GET | `/v1/emails:latest?count=1` | |
| POST | `/v1/emails:wait` | `{ "to", "subject", "query", "timeoutSeconds" }` max timeout 30s default 5s |
| GET | `/v1/stats` | |
| POST | `/v1/state:reset` | `{ "reason": "..." }` optional |
| GET | `/v1/version` | |
| GET | `/v1/capabilities` | |
| GET | `/v1/status` | smtp/http listen, counts, startedAt |
| GET | `/v1/config` | redacted effective config |
| GET | `/v1/schema/config` | JSON Schema |

Wait returns `200` with the email(s) or `504` problem+json `deadline_exceeded` if none matched. Does not mark read.

Relay: `POST /email/:id/relay` and `POST /email/:id/relay/:relayTo` (and `/api` twins). Success JSON `true`. Invalid `relayTo`: 400. Outgoing off: 500 with MailDev-style `{ "error": "..." }`.

## CORS

Configurable. Default: allow the UI origin (same origin when embedded). Lab may set `*`.

## Timeouts

ReadHeader, Read, Write, Idle must be set. Wait endpoint uses a short server-side cap independent of Write timeout (use hijack or a dedicated timeout budget).

## Compatibility tests required

- Unauthenticated `GET /email` → 401 when basic configured (lab smoke).
- Authed list contains a just-sent `Subject`.
- `/email/{id}` and `/api/email/{id}` return the same body.
- UI client paths under `/api` succeed without trailing-slash tricks.
- Relay: 500 MailDev-style error when outgoing is off; 200 JSON `true` and `relay-sink` receives the message when outgoing is configured.
- Invalid `relayTo` → 400. Unknown id → 404 `{ "error": "Email was not found" }`.
- Comparison-lab REST cases in [23-behavior-parity-matrix.md](23-behavior-parity-matrix.md) are the live oracle.
