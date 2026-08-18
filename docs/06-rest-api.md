# REST API

Status: Proposed normative behavior
Owners: REST, Application
Last reviewed: 2026-08-18 (COMPAT-001 + OBS-001 + SEC-001 + UI-001 + apply idempotency)
Related ADRs: 0004, 0005, 0007

Base: `/v1`. JSON unless noted. Errors: `Content-Type: application/problem+json`. Capability table: [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md). Generated OpenAPI: [api/openapi/v1.json](https://github.com/hilather/go-lab-maildev/blob/main/api/openapi/v1.json). `labmail serve` binds this listener from YAML `spec.listeners.management.address` (default `:1080`); `--management-listen ADDR|off` overrides.

Native `/v1` includes UI session (`POST/GET/DELETE /v1/session`). Auth is lab static bearer; HTTP Basic maps onto the same principal when `mode: bearer_and_basic`. COMPAT-001 mounts `/email`, `/healthz`, and `/config` on this same listener (`spec.listeners.management.compatEnabled`, default true). MCP is bearer-only.

When `spec.ui.enabled` is true (default), unmatched non-API GET/HEAD paths on this listener serve the embedded inbox SPA (`internal/web`). `/v1`, `/mcp`, `/email`, `/healthz`, `/config`, and `/.well-known` stay problem+json. `spec.ui.enabled: false` returns 404 for `/` and keeps REST/MCP up.

## Problem details

```json
{
  "type": "urn:labmail:error:not-found",
  "title": "Not Found",
  "status": 404,
  "detail": "message not found",
  "code": "not_found",
  "instance": "urn:labmail:request:01J…"
}
```

Domain codes and HTTP mapping (LabDNS `domainerr` table, mail-specific additions):

| Code | HTTP |
|---|---|
| `validation_failed` | 400 |
| `unauthenticated` | 401 |
| `forbidden` | 403 |
| `receive_only` | 403 |
| `not_found` | 404 |
| `method_not_allowed` | 405 |
| `revision_conflict` | 409 |
| `idempotency_conflict` | 409 |
| `store_full` | 409 (management); SMTP 452 |
| `store_over_new_cap` | 400 |
| `cursor_stale` | 400 |
| `rate_limited` | 429 |
| `timeout` | 504 |
| `internal_error` | 500 |

`application/problem+json` `code` **is** the table token (`cursor_stale`, `store_over_new_cap`, `validation_failed`, …). `type` is `urn:labmail:error:` plus that token with underscores turned to hyphens. Do **not** wrap a specific code as `code: validation_failed` with the real name only in `detail`.

## Auth and origin

Auth: `Authorization: Bearer <token>` or (when `bearer_and_basic`) `Authorization: Basic …`. Health live/ready skip auth. `X-Forwarded-For` is not trusted. No CORS headers. Loopback unauthenticated only when `mode: dev-loopback-unauth` (not the container default).

## Health and metrics

| Probe | Meaning | Auth |
|---|---|---|
| `GET /v1/health/live` | Process up | none |
| `GET /v1/health/ready` | SMTP bound **and** store initialized **and** (management bound or explicitly off). Does not require MCP clients or a non-empty inbox. | none |
| `GET /v1/status` `ready` | Same probe as `/v1/health/ready` when served over management HTTP | `mail.read` (stubbed open until SEC-001) |
| `GET /v1/metrics` | OpenMetrics text. **404 `not_found`** unless `spec.observability.metrics.publicPath` is `true`. | `mail.read` when enabled |

Ready becomes unready as soon as SMTP `Shutdown` begins (`Accepting()` is false), even while management is still draining. The process scrape listener (`spec.observability.metrics.listen`) is separate and unauthenticated; empty listen disables it.

**Origin (LabDNS wording, copied):** a **present** non-loopback `Origin` is rejected unless it is on `originAllowlist` (DNS-rebinding default-deny). **Missing Origin is allowed** for official SDK, curl, and MCPJungle (the gateway typically sends no Origin). Loopback Origins are those whose host is `localhost`, `127.0.0.1`, `::1`, or any RFC 6890 loopback; `http://localhost:1080` and `http://127.0.0.1:1080` are both loopback. Published LAN UI (`http://192.168.x.x:1080`) **must** list that origin in `originAllowlist` or the browser will 403.

Mutations accept `Idempotency-Key` and `If-Match` / body `expectedRevision` or `expectedStoreGeneration`. Plan/apply identity is `expectedRevision` + `force` + `reason` + operations. Idempotency LRU default 256; reset clears it.

## Message list (native)

`GET /v1/messages?to=&from=&subject=&subjectContains=&unread=&after=&before=&cursor=&limit=`

```json
{
  "revision": "sha256:…",
  "storeGeneration": 18,
  "items": [ { "id": "01J…", "subject": "…", "…" : "…" } ],
  "nextCursor": null
}
```

Default `limit=50`, max `200`. Sort: `receivedAt` descending, then id desc.

**Cursor:** opaque `base64url(id || uint64 storeGeneration || HMAC-SHA256)`. MAC key is **32 random bytes generated at process start**, never persisted, never logged, never a metric label. Reset/restart issues a new key (all cursors die — clients restart the list). If the generation embedded in the cursor ≠ current `storeGeneration`, return `400` with `code: cursor_stale` (not `validation_failed`); client must list from scratch. Mark-read does **not** invalidate cursors.

`MessageListItem` omits `raw`, `text`, `html` (only a `hasHTML` bool), and attachment bytes. Full `GET /v1/messages/{id}?markRead=false` (default) includes `text`, `html`, headers, envelope, attachment metadata. `markRead=true` is a write of the read bit only.

## Wait

`POST /v1/messages:wait`

```json
{
  "filter": {
    "subjectContains": "mcplab smoke",
    "to": "inbox@lab.test",
    "from": "",
    "after": "2026-08-17T00:00:00Z"
  },
  "timeout": "10s"
}
```

Returns the first matching message (full object) or `timeout`. Does not consume/delete. This is the agent replacement for the smoke poll loop. Config cap `store.maxWait: 60s`. Default request timeout `10s`.

## Extract

`POST /v1/messages/{id}:extract`

```json
{
  "urls": ["https://app.lab.test/verify?token=…"],
  "tokens": [
    { "kind": "otp_digits", "value": "482193", "context": "Your code is 482193" }
  ]
}
```

Frozen extract regexes (RE2; implement exactly; do not invent ML):

```
urlPlain   = (?i)\bhttps?://[^\s"'<>]+
urlAttr    = (?i)(?:href|src)\s*=\s*["'](https?://[^"']+)["']
otpNear    = (?i)(?:code|otp|pin|verify|token)[^\n]{0,40}\b(\d{4,8})\b
otpQuery   = (?i)(?:[?&](?:token|code)=)([A-Za-z0-9_-]{4,64})
```

`urlPlain` runs on `text`. `urlAttr` runs on `html`. Dedup URLs preserving first-seen order. `otpNear` and `otpQuery` run on both bodies; `kind` is `otp_digits` or `otp_query`. `context` is the matching line truncated to 120 runes. Cap 32 URLs and 16 tokens.

## Session (REST_ONLY)

```
POST   /v1/session     Authorization: Bearer | Basic
                       Set-Cookie: labmail_session=<opaque>; HttpOnly; SameSite=Lax; Path=/
                                   Secure iff management TLS
                       Body: { "csrf": "<32-byte hex>", "expiresAt": "…" }
GET    /v1/session     cookie or bearer → { "id", "role", "scopes", "expiresAt", "csrf"? }
                       csrf is returned for a valid cookie so the UI can recover
                       after reload; cookie-authenticated POST /v1/session still
                       requires X-LabMail-CSRF. Session JSON is Cache-Control: no-store.
DELETE /v1/session     clears cookie
```

Cookie mutations (any non-GET with `Cookie: labmail_session=…` and no `Authorization`) require header `X-LabMail-CSRF: <csrf>`. Mismatch → `403` `forbidden`. Session TTL default 12h, idle 4h. Cookie is REST-only; MCP never sees it.

## Events SSE (PARITY_DIFFERENT_BINDING)

`GET /v1/events/stream` (`Accept: text/event-stream`, scope `mail.read`):

```
event: mail.received
data: {"id":"01J…","subject":"…","storeGeneration":19}

event: mail.deleted
data: {"id":"01J…","storeGeneration":20}

event: store.wiped
data: {"storeGeneration":21}
```

Heartbeat comment every 15s. MCP `subscriptions/listen` on `labmail://messages` notifies **URI only**; clients pull bodies with `mail_messages_list`. Same handler; adapters differ only in framing.

## Preview

`GET /v1/messages/{id}/preview` response headers (frozen):

```
Content-Type: text/html; charset=utf-8
Content-Security-Policy: default-src 'none'; img-src data:; style-src 'unsafe-inline'
X-Content-Type-Options: nosniff
Content-Disposition: inline
```

**`cid:` rewrite (option a, the only rule that works):** the preview document inlines matching parts as `data:<contentType>;base64,…` URLs before it is served. Do **not** rewrite `cid:` to HTTP attachment paths. The iframe is a unique origin (no `allow-same-origin`, no `allow-scripts`), so it cannot mint `blob:` URLs and CSP `'self'` would not match LabMail’s host. `img-src data:` is therefore the only legal image source. Remote `http(s):` images stay broken (no tracking pixels). Parts larger than 2 MiB decoded, or missing `Content-ID`, are omitted (broken image), not fetched at runtime.

`GET /v1/messages/{id}/attachments/{attId}` (and compat `/email/:id/attachment/:filename`) remains **download**: `Content-Disposition: attachment` + `X-Content-Type-Options: nosniff`. It is not used by the preview document. Compat `GET /email/:id/html` uses the same CSP and `cid:` rewrite (`internal/preview`).

## Config plan/apply

See [docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md). `:plan` is dry-run. `:apply` requires `expectedRevision`.

## Compatibility promise

`/v1/*` is versioned; breaking change requires `/v2` or a documented flag day.
