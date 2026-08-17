# Error model

Status: Proposed normative
Last reviewed: 2026-08-17

Stable `code` values in `internal/domainerr`. Transports map to HTTP / JSON-RPC but must not invent parallel taxonomies.

| code | HTTP `/v1` | Meaning |
| --- | --- | --- |
| `invalid_argument` | 400 | Bad JSON, bad id charset, bad filter |
| `unauthenticated` | 401 | Missing/wrong basic or bearer |
| `permission_denied` | 403 | Token lacks scope |
| `not_found` | 404 | Unknown email id or attachment name |
| `already_exists` | 409 | Reserved (unused at GA) |
| `payload_too_large` | 413 | Attachment/MCP binary cap or body |
| `resource_exhausted` | 429 / SMTP 452 | Inbox or connection cap |
| `deadline_exceeded` | 504 | `emails.wait` timeout |
| `failed_precondition` | 500 on `/email` relay (MailDev); 409 or 400 on `/v1` | Outgoing not configured; prefer MailDev status on compat routes |
| `unimplemented` | 501 | Unused for relay (relay is implemented) |
| `unavailable` | 503 | Shutting down |
| `internal` | 500 | Bug |

MailDev UI routes keep `{ "error": "<message>" }` with the historical strings where tests/UI depend on them (`Email was not found`). `/v1` uses problem+json:

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "Email was not found",
  "code": "not_found",
  "emailId": "deadbeef"
}
```

MCP `error.data` includes `code` and safe `detail`. Never put secrets in `detail`.
